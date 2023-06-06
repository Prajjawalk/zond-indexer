package types

import (
	"database/sql"

	"github.com/jackc/pgtype"
	"github.com/shopspring/decimal"
)

// ChainHead is a struct to hold chain head data
type ChainHead struct {
	HeadSlot                   uint64
	HeadEpoch                  uint64
	HeadBlockRoot              []byte
	FinalizedSlot              uint64
	FinalizedEpoch             uint64
	FinalizedBlockRoot         []byte
	JustifiedSlot              uint64
	JustifiedEpoch             uint64
	JustifiedBlockRoot         []byte
	PreviousJustifiedSlot      uint64
	PreviousJustifiedEpoch     uint64
	PreviousJustifiedBlockRoot []byte
}

type FinalityCheckpoints struct {
	PreviousJustified struct {
		Epoch uint64 `json:"epoch"`
		Root  string `json:"root"`
	} `json:"previous_justified"`
	CurrentJustified struct {
		Epoch uint64 `json:"epoch"`
		Root  string `json:"root"`
	} `json:"current_justified"`
	Finalized struct {
		Epoch uint64 `json:"epoch"`
		Root  string `json:"root"`
	} `json:"finalized"`
}

// Validator is a struct to hold validator data
type Validator struct {
	Index                      uint64 `db:"validatorindex"`
	PublicKey                  []byte `db:"pubkey"`
	PublicKeyHex               string `db:"pubkeyhex"`
	Balance                    uint64 `db:"balance"`
	EffectiveBalance           uint64 `db:"effectivebalance"`
	Slashed                    bool   `db:"slashed"`
	ActivationEligibilityEpoch uint64 `db:"activationeligibilityepoch"`
	ActivationEpoch            uint64 `db:"activationepoch"`
	ExitEpoch                  uint64 `db:"exitepoch"`
	WithdrawableEpoch          uint64 `db:"withdrawableepoch"`
	WithdrawalCredentials      []byte `db:"withdrawalcredentials"`

	BalanceActivation sql.NullInt64 `db:"balanceactivation"`
	Status            string        `db:"status"`

	LastAttestationSlot sql.NullInt64 `db:"lastattestationslot"`
	LastProposalSlot    sql.NullInt64 `db:"lastproposalslot"`
}

type SyncAggregate struct {
	SyncCommitteeValidators    []uint64
	SyncCommitteeBits          []byte
	SyncCommitteeSignature     []byte
	SyncAggregateParticipation float64
}

// Block is a struct to hold block data
type Block struct {
	Status                     uint64
	Proposer                   uint64
	BlockRoot                  []byte
	Slot                       uint64
	ParentRoot                 []byte
	StateRoot                  []byte
	Signature                  []byte
	RandaoReveal               []byte
	Graffiti                   []byte
	Eth1Data                   *Eth1Data
	BodyRoot                   []byte
	ProposerSlashings          []*ProposerSlashing
	AttesterSlashings          []*AttesterSlashing
	Attestations               []*Attestation
	Deposits                   []*Deposit
	VoluntaryExits             []*VoluntaryExit
	SyncAggregate              *SyncAggregate    // warning: sync aggregate may be nil, for phase0 blocks
	ExecutionPayload           *ExecutionPayload // warning: payload may be nil, for phase0/altair blocks
	Canonical                  bool
	SignedBLSToExecutionChange []*SignedBLSToExecutionChange
}

type SignedBLSToExecutionChange struct {
	Message   BLSToExecutionChange
	Signature []byte
}

type BLSToExecutionChange struct {
	Validatorindex uint64
	BlsPubkey      []byte
	Address        []byte
}

type Transaction struct {
	Raw []byte
	// Note: below values may be nil/0 if Raw fails to decode into a valid transaction
	TxHash       []byte
	AccountNonce uint64
	// big endian
	Price     []byte
	GasLimit  uint64
	Sender    []byte
	Recipient []byte
	// big endian
	Amount  []byte
	Payload []byte

	MaxPriorityFeePerGas uint64
	MaxFeePerGas         uint64
}

type ExecutionPayload struct {
	ParentHash    []byte
	FeeRecipient  []byte
	StateRoot     []byte
	ReceiptsRoot  []byte
	LogsBloom     []byte
	Random        []byte
	BlockNumber   uint64
	GasLimit      uint64
	GasUsed       uint64
	Timestamp     uint64
	ExtraData     []byte
	BaseFeePerGas uint64
	BlockHash     []byte
	Transactions  []*Transaction
	Withdrawals   []*Withdrawals
}

type Withdrawals struct {
	Slot           uint64 `json:"slot,omitempty"`
	BlockRoot      []byte `json:"blockroot,omitempty"`
	Index          uint64 `json:"index"`
	ValidatorIndex uint64 `json:"validatorindex"`
	Address        []byte `json:"address"`
	Amount         uint64 `json:"amount"`
}

type WithdrawalsByEpoch struct {
	Epoch          uint64
	ValidatorIndex uint64
	Amount         uint64
}

type WithdrawalsNotification struct {
	Slot           uint64 `json:"slot,omitempty"`
	Index          uint64 `json:"index"`
	ValidatorIndex uint64 `json:"validatorindex"`
	Address        []byte `json:"address"`
	Amount         uint64 `json:"amount"`
	Pubkey         []byte `json:"pubkey"`
}

// Eth1Data is a struct to hold the ETH1 data
type Eth1Data struct {
	DepositRoot  []byte
	DepositCount uint64
	BlockHash    []byte
}

// ProposerSlashing is a struct to hold proposer slashing data
type ProposerSlashing struct {
	ProposerIndex uint64
	Header1       *Block
	Header2       *Block
}

// AttesterSlashing is a struct to hold attester slashing
type AttesterSlashing struct {
	Attestation1 *IndexedAttestation
	Attestation2 *IndexedAttestation
}

// IndexedAttestation is a struct to hold indexed attestation data
type IndexedAttestation struct {
	Data             *AttestationData
	AttestingIndices []uint64
	Signature        []byte
}

// Attestation is a struct to hold attestation header data
type Attestation struct {
	AggregationBits []byte
	Attesters       []uint64
	Data            *AttestationData
	Signature       []byte
}

// AttestationData to hold attestation detail data
type AttestationData struct {
	Slot            uint64
	CommitteeIndex  uint64
	BeaconBlockRoot []byte
	Source          *Checkpoint
	Target          *Checkpoint
}

// Checkpoint is a struct to hold checkpoint data
type Checkpoint struct {
	Epoch uint64
	Root  []byte
}

// Deposit is a struct to hold deposit data
type Deposit struct {
	Proof                 [][]byte
	PublicKey             []byte
	WithdrawalCredentials []byte
	Amount                uint64
	Signature             []byte
}

// VoluntaryExit is a struct to hold voluntary exit data
type VoluntaryExit struct {
	Epoch          uint64
	ValidatorIndex uint64
	Signature      []byte
}

// MinimalBlock is a struct to hold minimal block data
type MinimalBlock struct {
	Epoch      uint64 `db:"epoch"`
	Slot       uint64 `db:"slot"`
	BlockRoot  []byte `db:"blockroot"`
	ParentRoot []byte `db:"parentroot"`
	Canonical  bool   `db:"-"`
}

// CanonBlock is a struct to hold canon block data
type CanonBlock struct {
	BlockRoot []byte `db:"blockroot"`
	Slot      uint64 `db:"slot"`
	Canonical bool   `db:"-"`
}

// ValidatorQueue is a struct to hold validator queue data
type ValidatorQueue struct {
	Activating uint64
	Exiting    uint64
}

// EpochData is a struct to hold epoch data
type EpochData struct {
	Epoch                   uint64
	Validators              []*Validator
	ValidatorAssignmentes   *EpochAssignments
	Blocks                  map[uint64]map[string]*Block
	EpochParticipationStats *ValidatorParticipation
}

// ValidatorParticipation is a struct to hold validator participation data
type ValidatorParticipation struct {
	Epoch                   uint64
	GlobalParticipationRate float32
	VotedEther              uint64
	EligibleEther           uint64
}

type WeiString struct {
	pgtype.Numeric
}

// EpochAssignments is a struct to hold epoch assignment data
type EpochAssignments struct {
	ProposerAssignments map[uint64]uint64
	AttestorAssignments map[string]uint64
	SyncAssignments     []uint64
}

// EthStoreDay is a struct to hold performance data for a specific beaconchain-day.
// All fields use Gwei unless specified otherwise by the field name
type EthStoreDay struct {
	Pool                   string          `db:"pool"`
	Day                    uint64          `db:"day"`
	EffectiveBalancesSum   decimal.Decimal `db:"effective_balances_sum_wei"`
	StartBalancesSum       decimal.Decimal `db:"start_balances_sum_wei"`
	EndBalancesSum         decimal.Decimal `db:"end_balances_sum_wei"`
	DepositsSum            decimal.Decimal `db:"deposits_sum_wei"`
	TxFeesSumWei           decimal.Decimal `db:"tx_fees_sum_wei"`
	ConsensusRewardsSumWei decimal.Decimal `db:"consensus_rewards_sum_wei"`
	TotalRewardsWei        decimal.Decimal `db:"total_rewards_wei"`
	APR                    decimal.Decimal `db:"apr"`
}
