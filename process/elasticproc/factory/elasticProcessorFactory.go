package factory

import (
	"encoding/hex"
	"fmt"
	"github.com/multiversx/mx-chain-core-go/core"
	"github.com/multiversx/mx-chain-core-go/hashing"
	"github.com/multiversx/mx-chain-core-go/marshal"
	"github.com/multiversx/mx-chain-es-indexer-go/config"
	"github.com/multiversx/mx-chain-es-indexer-go/process/dataindexer"
	elasticIndexer "github.com/multiversx/mx-chain-es-indexer-go/process/dataindexer"
	"github.com/multiversx/mx-chain-es-indexer-go/process/elasticproc"
	"github.com/multiversx/mx-chain-es-indexer-go/process/elasticproc/accounts"
	blockProc "github.com/multiversx/mx-chain-es-indexer-go/process/elasticproc/block"
	"github.com/multiversx/mx-chain-es-indexer-go/process/elasticproc/converters"
	"github.com/multiversx/mx-chain-es-indexer-go/process/elasticproc/logsevents"
	"github.com/multiversx/mx-chain-es-indexer-go/process/elasticproc/miniblocks"
	"github.com/multiversx/mx-chain-es-indexer-go/process/elasticproc/operations"
	"github.com/multiversx/mx-chain-es-indexer-go/process/elasticproc/statistics"
	"github.com/multiversx/mx-chain-es-indexer-go/process/elasticproc/templatesAndPolicies"
	"github.com/multiversx/mx-chain-es-indexer-go/process/elasticproc/transactions"
	"github.com/multiversx/mx-chain-es-indexer-go/process/elasticproc/validators"
	"strings"
)

// ArgElasticProcessorFactory is struct that is used to store all components that are needed to create an elastic processor factory
type ArgElasticProcessorFactory struct {
	Marshalizer              marshal.Marshalizer
	Hasher                   hashing.Hasher
	AddressPubkeyConverter   core.PubkeyConverter
	ValidatorPubkeyConverter core.PubkeyConverter
	DBClient                 elasticproc.DatabaseClientHandler
	EnabledIndexes           []string
	Version                  string
	Denomination             int
	BulkRequestMaxSize       int
	UseKibana                bool
	ImportDB                 bool
	DRWAAuthorizedEmitters   []string
	MRVAuthorizedEmitters    []string
	EnableEpochsConfig       config.EnableEpochsConfig
}

// CreateElasticProcessor will create a new instance of ElasticProcessor
func CreateElasticProcessor(arguments ArgElasticProcessorFactory) (dataindexer.ElasticProcessor, error) {
	templatesAndPoliciesReader := templatesAndPolicies.NewTemplatesAndPolicyReader()

	enabledIndexesMap := make(map[string]struct{})
	for _, index := range arguments.EnabledIndexes {
		enabledIndexesMap[index] = struct{}{}
	}
	if len(enabledIndexesMap) == 0 {
		return nil, dataindexer.ErrEmptyEnabledIndexes
	}

	drwaAuthorizedEmitters, err := parseConfiguredEmitters(arguments.AddressPubkeyConverter, arguments.DRWAAuthorizedEmitters, "DRWA")
	if err != nil {
		return nil, err
	}
	if requiresDRWAEmitterAuthority(enabledIndexesMap) && len(drwaAuthorizedEmitters) == 0 {
		return nil, fmt.Errorf("DRWA indices are enabled but no [config.drwa].authorized-emitters are configured")
	}
	mrvAuthorizedEmitters, err := parseConfiguredEmitters(arguments.AddressPubkeyConverter, arguments.MRVAuthorizedEmitters, "MRV")
	if err != nil {
		return nil, err
	}
	if requiresMRVEmitterAuthority(enabledIndexesMap) && len(mrvAuthorizedEmitters) == 0 {
		return nil, fmt.Errorf("MRV indices are enabled but no [config.mrv].authorized-emitters are configured")
	}

	balanceConverter, err := converters.NewBalanceConverter(arguments.Denomination)
	if err != nil {
		return nil, err
	}

	accountsProc, err := accounts.NewAccountsProcessor(
		arguments.AddressPubkeyConverter,
		balanceConverter,
	)
	if err != nil {
		return nil, err
	}

	blockProcHandler, err := blockProc.NewBlockProcessor(arguments.Hasher, arguments.Marshalizer, arguments.ValidatorPubkeyConverter)
	if err != nil {
		return nil, err
	}

	miniblocksProc, err := miniblocks.NewMiniblocksProcessor(arguments.Hasher, arguments.Marshalizer)
	if err != nil {
		return nil, err
	}
	validatorsProc, err := validators.NewValidatorsProcessor(arguments.ValidatorPubkeyConverter, arguments.BulkRequestMaxSize)
	if err != nil {
		return nil, err
	}

	generalInfoProc := statistics.NewStatisticsProcessor()

	argsTxsProc := &transactions.ArgsTransactionProcessor{
		AddressPubkeyConverter: arguments.AddressPubkeyConverter,
		Hasher:                 arguments.Hasher,
		Marshalizer:            arguments.Marshalizer,
		BalanceConverter:       balanceConverter,
		EnableEpochsConfig:     arguments.EnableEpochsConfig,
	}
	txsProc, err := transactions.NewTransactionsProcessor(argsTxsProc)
	if err != nil {
		return nil, err
	}

	argsLogsAndEventsProc := logsevents.ArgsLogsAndEventsProcessor{
		PubKeyConverter:        arguments.AddressPubkeyConverter,
		Marshalizer:            arguments.Marshalizer,
		BalanceConverter:       balanceConverter,
		Hasher:                 arguments.Hasher,
		DRWAAuthorizedEmitters: drwaAuthorizedEmitters,
		MRVAuthorizedEmitters:  mrvAuthorizedEmitters,
	}
	logsAndEventsProc, err := logsevents.NewLogsAndEventsProcessor(argsLogsAndEventsProc)
	if err != nil {
		return nil, err
	}

	operationsProc, err := operations.NewOperationsProcessor()
	if err != nil {
		return nil, err
	}

	args := &elasticproc.ArgElasticProcessor{
		BulkRequestMaxSize: arguments.BulkRequestMaxSize,
		TransactionsProc:   txsProc,
		AccountsProc:       accountsProc,
		BlockProc:          blockProcHandler,
		MiniblocksProc:     miniblocksProc,
		ValidatorsProc:     validatorsProc,
		StatisticsProc:     generalInfoProc,
		LogsAndEventsProc:  logsAndEventsProc,
		DBClient:           arguments.DBClient,
		EnabledIndexes:     enabledIndexesMap,
		UseKibana:          arguments.UseKibana,
		OperationsProc:     operationsProc,
		ImportDB:           arguments.ImportDB,
		Version:            arguments.Version,
		MappingsHandler:    templatesAndPoliciesReader,
	}

	return elasticproc.NewElasticProcessor(args)
}

func parseConfiguredEmitters(converter core.PubkeyConverter, configured []string, label string) ([][]byte, error) {
	emitters := make([][]byte, 0, len(configured))
	for _, raw := range configured {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}

		if strings.HasPrefix(strings.ToLower(value), "0x") {
			decoded, err := hex.DecodeString(value[2:])
			if err != nil {
				return nil, fmt.Errorf("invalid %s authorized emitter %q: %w", label, raw, err)
			}
			emitters = append(emitters, decoded)
			continue
		}

		decoded, err := converter.Decode(value)
		if err != nil {
			return nil, fmt.Errorf("invalid %s authorized emitter %q: %w", label, raw, err)
		}
		emitters = append(emitters, decoded)
	}

	return emitters, nil
}

func requiresMRVEmitterAuthority(enabledIndexes map[string]struct{}) bool {
	_, ok := enabledIndexes[elasticIndexer.MrvAnchoredProofsIndex]
	return ok
}

func requiresDRWAEmitterAuthority(enabledIndexes map[string]struct{}) bool {
	for _, index := range []string{
		elasticIndexer.DrwaDenialsIndex,
		elasticIndexer.DrwaIdentitiesIndex,
		elasticIndexer.DrwaHolderComplianceIndex,
		elasticIndexer.DrwaAttestationsIndex,
		elasticIndexer.DrwaTokenPoliciesIndex,
		elasticIndexer.DrwaControlEventsIndex,
	} {
		if _, ok := enabledIndexes[index]; ok {
			return true
		}
	}

	return false
}
