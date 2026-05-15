package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/multiversx/mx-chain-es-indexer-go/core"
	"github.com/multiversx/mx-chain-es-indexer-go/process/dataindexer"
)

func exists(res *esapi.Response, err error) bool {
	defer func() {
		if res != nil && res.Body != nil {
			err = res.Body.Close()
			if err != nil {
				log.Warn("elasticClient.exists", "could not close body: ", core.SanitizeLogError(err))
			}
		}
	}()

	if err != nil {
		log.Warn("elasticClient.IndexExists", "could not check index on the elastic nodes:", core.SanitizeLogError(err))
		return false
	}

	switch res.StatusCode {
	case http.StatusOK:
		return true
	case http.StatusNotFound:
		return false
	default:
		log.Warn("elasticClient.exists", "invalid status code returned by the elastic nodes:", res.StatusCode)
		return false
	}
}

func loadResponseBody(body io.ReadCloser, dest interface{}) error {
	if body == nil {
		return nil
	}
	if dest == nil {
		_, err := io.Copy(io.Discard, body)
		return err
	}

	err := json.NewDecoder(body).Decode(dest)
	return err
}

func elasticDefaultErrorResponseHandler(res *esapi.Response) error {
	responseBody := map[string]interface{}{}
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("%w cannot read elastic response body bytes", err)
	}

	err = json.Unmarshal(bodyBytes, &responseBody)
	if err != nil {
		errToReturn := err
		isBackOffError := strings.Contains(string(bodyBytes), fmt.Sprintf("%d", http.StatusForbidden)) ||
			strings.Contains(string(bodyBytes), fmt.Sprintf("%d", http.StatusTooManyRequests))
		if isBackOffError {
			errToReturn = dataindexer.ErrBackOff
		}

		return fmt.Errorf("%w, cannot unmarshal elastic response body to map[string]interface{}, "+
			"decode error: %s, body response: %s",
			errToReturn,
			core.SanitizeLogError(err),
			core.SanitizeLogValue(string(bodyBytes)),
		)
	}

	if res.IsError() {
		if errIsAlreadyExists(responseBody) {
			return nil
		}
		if isErrAliasAlreadyExists(responseBody) {
			log.Debug("alias already exists", "response", responseBody)
			return nil
		}
	}
	if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusCreated {
		return nil
	}

	return fmt.Errorf("error while parsing the response: code returned: %v, body: %v, bodyBytes: %v",
		res.StatusCode, responseBody, string(bodyBytes))
}

func elasticBulkRequestResponseHandler(res *esapi.Response) error {
	if res.IsError() {
		return fmt.Errorf("%s", res.String())
	}
	defer func() {
		_ = res.Body.Close()
	}()

	var response struct {
		Errors bool            `json:"errors"`
		Items  json.RawMessage `json:"items"`
	}

	err := json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return fmt.Errorf("cannot decode elastic response body: %w", err)
	}

	if !response.Errors {
		return nil
	}

	return extractErrorFromBulkItems(response.Items)
}

func extractErrorFromBulkItems(itemsBytes []byte) error {
	var items []struct {
		ItemIndex  *Item `json:"index"`
		ItemUpdate *Item `json:"update"`
	}

	err := json.Unmarshal(itemsBytes, &items)
	if err != nil {
		return fmt.Errorf("cannot unmarshal bulk items: %w", err)
	}

	count := 0
	errorsString := ""
	for _, item := range items {
		var selectedItem Item

		switch {
		case item.ItemIndex != nil:
			selectedItem = *item.ItemIndex
		case item.ItemUpdate != nil:
			selectedItem = *item.ItemUpdate
		}

		log.Trace("worked on", "index", selectedItem.Index,
			"_id", selectedItem.ID,
			"result", selectedItem.Result,
			"status", selectedItem.Status,
		)

		if selectedItem.Status < http.StatusBadRequest {
			continue
		}

		count++
		errMap := map[string]interface{}{
			"index":      selectedItem.Index,
			"id":         selectedItem.ID,
			"statusCode": selectedItem.Status,
			"errorType":  selectedItem.Error.Type,
			"reason":     selectedItem.Error.Reason,
			"causedBy": map[string]interface{}{
				"type":         selectedItem.Error.Cause.Type,
				"reason":       selectedItem.Error.Cause.Reason,
				"script_stack": selectedItem.Error.Cause.ScriptStack,
				"script":       selectedItem.Error.Cause.Script,
			},
		}

		marshaledErr, errMarshal := json.Marshal(errMap)
		if errMarshal != nil {
			log.Warn("cannot marshal bulk item error", "error", errMarshal)
			continue
		}

		errorsString += string(marshaledErr) + "\n"

		if count == numOfErrorsToExtractBulkResponse {
			break
		}
	}
	if errorsString == "" {
		return nil
	}

	return fmt.Errorf("%s", errorsString)
}

func errIsAlreadyExists(response map[string]interface{}) bool {
	alreadyExistsMessage := "resource_already_exists_exception"
	errKey := "error"
	typeKey := "type"

	errMapI, ok := response[errKey]
	if !ok {
		return false
	}

	errMap, ok := errMapI.(map[string]interface{})
	if !ok {
		return false
	}

	existsString, ok := errMap[typeKey].(string)
	if !ok {
		return false
	}

	return existsString == alreadyExistsMessage
}

func isErrAliasAlreadyExists(response map[string]interface{}) bool {
	aliasExistsMessage := "invalid_alias_name_exception"
	errKey := "error"
	typeKey := "type"

	errMapI, ok := response[errKey]
	if !ok {
		return false
	}

	errMap, ok := errMapI.(map[string]interface{})
	if !ok {
		return false
	}

	existsString, ok := errMap[typeKey].(string)
	if !ok {
		return false
	}

	return existsString == aliasExistsMessage
}

/**
 * parseResponse will check and load the elastic/kibana api response into the destination objectsMap. Custom errorHandler
 *  can be passed for special requests that want to handle StatusCode != 200. Every responseErrorHandler
 *  implementation should call loadResponseBody or consume the response body in order to be able to
 *  reuse persistent TCP connections: https://github.com/elastic/go-elasticsearch#usage
 */
func parseResponse(res *esapi.Response, dest interface{}, errorHandler responseErrorHandler) error {
	defer func() {
		if res != nil && res.Body != nil {
			err := res.Body.Close()
			if err != nil {
				log.Warn("elasticClient.parseResponse",
					"could not close body", core.SanitizeLogError(err))
			}
		}
	}()

	if errorHandler == nil {
		errorHandler = elasticDefaultErrorResponseHandler
	}

	if res.StatusCode != http.StatusOK {
		return errorHandler(res)
	}

	err := loadResponseBody(res.Body, dest)
	if err != nil {
		log.Warn("elasticClient.parseResponse",
			"could not load response body:", core.SanitizeLogError(err))
		return dataindexer.ErrBackOff
	}

	return nil
}
