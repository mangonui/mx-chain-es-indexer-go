package factory

import (
	"testing"

	"github.com/multiversx/mx-chain-es-indexer-go/mock"
	"github.com/multiversx/mx-chain-es-indexer-go/process/dataindexer"
	"github.com/stretchr/testify/require"
)

func TestCreateElasticProcessor(t *testing.T) {

	args := ArgElasticProcessorFactory{
		Marshalizer:              &mock.MarshalizerMock{},
		Hasher:                   &mock.HasherMock{},
		AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
		ValidatorPubkeyConverter: &mock.PubkeyConverterMock{},
		DBClient:                 &mock.DatabaseWriterStub{},
		EnabledIndexes:           []string{"blocks"},
		Denomination:             1,
		UseKibana:                false,
	}

	ep, err := CreateElasticProcessor(args)
	require.Nil(t, err)
	require.NotNil(t, ep)
}

func TestCreateElasticProcessor_WithDRWAIndexesRequiresAuthorizedEmitters(t *testing.T) {
	args := ArgElasticProcessorFactory{
		Marshalizer:              &mock.MarshalizerMock{},
		Hasher:                   &mock.HasherMock{},
		AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
		ValidatorPubkeyConverter: &mock.PubkeyConverterMock{},
		DBClient:                 &mock.DatabaseWriterStub{},
		EnabledIndexes:           []string{dataindexer.DrwaIdentitiesIndex},
		Denomination:             1,
		UseKibana:                false,
	}

	ep, err := CreateElasticProcessor(args)
	require.Nil(t, ep)
	require.Error(t, err)
	require.Contains(t, err.Error(), "authorized-emitters")
}

func TestCreateElasticProcessor_WithDRWAIndexesAndAuthorizedEmittersWorks(t *testing.T) {
	args := ArgElasticProcessorFactory{
		Marshalizer:              &mock.MarshalizerMock{},
		Hasher:                   &mock.HasherMock{},
		AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
		ValidatorPubkeyConverter: &mock.PubkeyConverterMock{},
		DBClient:                 &mock.DatabaseWriterStub{},
		EnabledIndexes:           []string{dataindexer.DrwaIdentitiesIndex},
		Denomination:             1,
		UseKibana:                false,
		DRWAAuthorizedEmitters:   []string{"0x1111111111111111111111111111111111111111111111111111111111111111"},
	}

	ep, err := CreateElasticProcessor(args)
	require.NoError(t, err)
	require.NotNil(t, ep)
}

func TestCreateElasticProcessor_WithMRVIndexesRequiresAuthorizedEmitters(t *testing.T) {
	args := ArgElasticProcessorFactory{
		Marshalizer:              &mock.MarshalizerMock{},
		Hasher:                   &mock.HasherMock{},
		AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
		ValidatorPubkeyConverter: &mock.PubkeyConverterMock{},
		DBClient:                 &mock.DatabaseWriterStub{},
		EnabledIndexes:           []string{dataindexer.MrvAnchoredProofsIndex},
		Denomination:             1,
		UseKibana:                false,
	}

	ep, err := CreateElasticProcessor(args)
	require.Nil(t, ep)
	require.Error(t, err)
	require.Contains(t, err.Error(), "[config.mrv].authorized-emitters")
}

func TestCreateElasticProcessor_WithMRVIndexesAndAuthorizedEmittersWorks(t *testing.T) {
	args := ArgElasticProcessorFactory{
		Marshalizer:              &mock.MarshalizerMock{},
		Hasher:                   &mock.HasherMock{},
		AddressPubkeyConverter:   mock.NewPubkeyConverterMock(32),
		ValidatorPubkeyConverter: &mock.PubkeyConverterMock{},
		DBClient:                 &mock.DatabaseWriterStub{},
		EnabledIndexes:           []string{dataindexer.MrvAnchoredProofsIndex},
		Denomination:             1,
		UseKibana:                false,
		MRVAuthorizedEmitters:    []string{"0x1111111111111111111111111111111111111111111111111111111111111111"},
	}

	ep, err := CreateElasticProcessor(args)
	require.NoError(t, err)
	require.NotNil(t, ep)
}
