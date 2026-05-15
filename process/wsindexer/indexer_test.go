package wsindexer

import (
	"errors"
	"testing"

	"github.com/multiversx/mx-chain-core-go/data/outport"
	"github.com/multiversx/mx-chain-core-go/marshal"
	"github.com/multiversx/mx-chain-es-indexer-go/core/request"
	"github.com/multiversx/mx-chain-es-indexer-go/metrics"
	"github.com/multiversx/mx-chain-es-indexer-go/process/dataindexer"
	"github.com/stretchr/testify/require"
)

type dataIndexerStub struct {
	saveBlockCalled          func(outportBlock *outport.OutportBlock) error
	setCurrentSettingsCalled func(settings outport.OutportConfig) error
	closeCalled              func() error
}

func (stub *dataIndexerStub) SaveBlock(outportBlock *outport.OutportBlock) error {
	if stub.saveBlockCalled != nil {
		return stub.saveBlockCalled(outportBlock)
	}
	return nil
}

func (stub *dataIndexerStub) RevertIndexedBlock(_ *outport.BlockData) error {
	return nil
}

func (stub *dataIndexerStub) SaveRoundsInfo(_ *outport.RoundsInfo) error {
	return nil
}

func (stub *dataIndexerStub) SaveValidatorsPubKeys(_ *outport.ValidatorsPubKeys) error {
	return nil
}

func (stub *dataIndexerStub) SaveValidatorsRating(_ *outport.ValidatorsRating) error {
	return nil
}

func (stub *dataIndexerStub) SaveAccounts(_ *outport.Accounts) error {
	return nil
}

func (stub *dataIndexerStub) FinalizedBlock(_ *outport.FinalizedBlock) error {
	return nil
}

func (stub *dataIndexerStub) SetCurrentSettings(settings outport.OutportConfig) error {
	if stub.setCurrentSettingsCalled != nil {
		return stub.setCurrentSettingsCalled(settings)
	}
	return nil
}

func (stub *dataIndexerStub) Close() error {
	if stub.closeCalled != nil {
		return stub.closeCalled()
	}
	return nil
}

func (stub *dataIndexerStub) IsInterfaceNil() bool {
	return stub == nil
}

type statusMetricsStub struct {
	addIndexingDataCalled func(args metrics.ArgsAddIndexingData)
}

func (stub *statusMetricsStub) AddIndexingData(args metrics.ArgsAddIndexingData) {
	if stub.addIndexingDataCalled != nil {
		stub.addIndexingDataCalled(args)
	}
}

func (stub *statusMetricsStub) GetMetrics() map[string]*request.MetricsResponse {
	return nil
}

func (stub *statusMetricsStub) GetMetricsForPrometheus() string {
	return ""
}

func (stub *statusMetricsStub) IsInterfaceNil() bool {
	return stub == nil
}

func TestNewIndexer_NilArgsShouldErr(t *testing.T) {
	t.Run("nil marshaller", func(t *testing.T) {
		idx, err := NewIndexer(ArgsIndexer{
			DataIndexer:   &dataIndexerStub{},
			StatusMetrics: &statusMetricsStub{},
		})

		require.Nil(t, idx)
		require.Equal(t, dataindexer.ErrNilMarshalizer, err)
	})

	t.Run("nil data indexer", func(t *testing.T) {
		idx, err := NewIndexer(ArgsIndexer{
			Marshaller:    &marshal.GogoProtoMarshalizer{},
			StatusMetrics: &statusMetricsStub{},
		})

		require.Nil(t, idx)
		require.Equal(t, errNilDataIndexer, err)
	})
}

func TestIndexer_ProcessPayloadShouldDispatchAndRecordMetrics(t *testing.T) {
	marshaller := &marshal.GogoProtoMarshalizer{}
	payload, err := marshaller.Marshal(&outport.OutportBlock{ShardID: 7})
	require.Nil(t, err)

	wasSaved := false
	var recordedMetrics metrics.ArgsAddIndexingData
	idx, err := NewIndexer(ArgsIndexer{
		Marshaller: marshaller,
		DataIndexer: &dataIndexerStub{
			saveBlockCalled: func(outportBlock *outport.OutportBlock) error {
				wasSaved = true
				require.Equal(t, uint32(7), outportBlock.ShardID)
				return nil
			},
		},
		StatusMetrics: &statusMetricsStub{
			addIndexingDataCalled: func(args metrics.ArgsAddIndexingData) {
				recordedMetrics = args
			},
		},
	})
	require.Nil(t, err)

	err = idx.ProcessPayload(payload, outport.TopicSaveBlock, 1)

	require.Nil(t, err)
	require.True(t, wasSaved)
	require.False(t, recordedMetrics.GotError)
	require.Equal(t, uint64(len(payload)), recordedMetrics.MessageLen)
	require.Equal(t, outport.TopicSaveBlock+"_7", recordedMetrics.Topic)
}

func TestIndexer_ProcessPayloadShouldIgnoreUnknownTopic(t *testing.T) {
	idx, err := NewIndexer(ArgsIndexer{
		Marshaller:    &marshal.GogoProtoMarshalizer{},
		DataIndexer:   &dataIndexerStub{},
		StatusMetrics: &statusMetricsStub{},
	})
	require.Nil(t, err)

	err = idx.ProcessPayload([]byte("not decoded"), "unknown-topic", 1)

	require.Nil(t, err)
}

func TestIndexer_ProcessPayloadShouldPropagateActionErrorAndRecordIt(t *testing.T) {
	expectedErr := errors.New("save failed")
	marshaller := &marshal.GogoProtoMarshalizer{}
	payload, err := marshaller.Marshal(&outport.OutportBlock{ShardID: 3})
	require.Nil(t, err)

	var recordedMetrics metrics.ArgsAddIndexingData
	idx, err := NewIndexer(ArgsIndexer{
		Marshaller: marshaller,
		DataIndexer: &dataIndexerStub{
			saveBlockCalled: func(_ *outport.OutportBlock) error {
				return expectedErr
			},
		},
		StatusMetrics: &statusMetricsStub{
			addIndexingDataCalled: func(args metrics.ArgsAddIndexingData) {
				recordedMetrics = args
			},
		},
	})
	require.Nil(t, err)

	err = idx.ProcessPayload(payload, outport.TopicSaveBlock, 1)

	require.Equal(t, expectedErr, err)
	require.True(t, recordedMetrics.GotError)
	require.Equal(t, outport.TopicSaveBlock+"_3", recordedMetrics.Topic)
}