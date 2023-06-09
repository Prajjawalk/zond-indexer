package utils

import (
	"bytes"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Prajjawalk/zond-indexer/config"
	"github.com/Prajjawalk/zond-indexer/price"
	"github.com/Prajjawalk/zond-indexer/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/kelseyhightower/envconfig"
	"github.com/lib/pq"
	"github.com/mvdan/xurls"
	"github.com/prysmaticlabs/prysm/v3/beacon-chain/core/signing"
	prysm_params "github.com/prysmaticlabs/prysm/v3/config/params"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"gopkg.in/yaml.v2"
)

// Config is the globally accessible configuration
var Config *types.Config
var eth1AddressRE = regexp.MustCompile("^(0x)?[0-9a-fA-F]{40}$")
var ErrRateLimit = errors.New("## RATE LIMIT ##")

func readConfigEnv(cfg *types.Config) error {
	return envconfig.Process("", cfg)
}

func readConfigSecrets(cfg *types.Config) error {
	return ProcessSecrets(cfg)
}

// ReadConfig will process a configuration
func ReadConfig(cfg *types.Config, path string) error {

	err := readConfigFile(cfg, path)
	if err != nil {
		return err
	}

	readConfigEnv(cfg)
	err = readConfigSecrets(cfg)
	if err != nil {
		return err
	}

	if cfg.Chain.ConfigPath == "" {
		// var prysmParamsConfig *prysmParams.BeaconChainConfig
		switch cfg.Chain.Name {
		case "mainnet":
			err = yaml.Unmarshal([]byte(config.MainnetChainYml), &cfg.Chain.Config)
		case "prater":
			err = yaml.Unmarshal([]byte(config.PraterChainYml), &cfg.Chain.Config)
		// case "ropsten":
		// 	err = yaml.Unmarshal([]byte(config.RopstenChainYml), &cfg.Chain.Config)
		// case "sepolia":
		// 	err = yaml.Unmarshal([]byte(config.SepoliaChainYml), &cfg.Chain.Config)
		// case "gnosis":
		// 	err = yaml.Unmarshal([]byte(config.GnosisChainYml), &cfg.Chain.Config)
		default:
			return fmt.Errorf("tried to set known chain-config, but unknown chain-name")
		}
		if err != nil {
			return err
		}
		// err = prysmParams.SetActive(prysmParamsConfig)
		// if err != nil {
		// 	return fmt.Errorf("error setting chainConfig (%v) for prysmParams: %w", cfg.Chain.Name, err)
		// }
	} else {
		f, err := os.Open(cfg.Chain.ConfigPath)
		if err != nil {
			return fmt.Errorf("error opening Chain Config file %v: %w", cfg.Chain.ConfigPath, err)
		}
		var chainConfig *types.ChainConfig
		decoder := yaml.NewDecoder(f)
		err = decoder.Decode(&chainConfig)
		if err != nil {
			return fmt.Errorf("error decoding Chain Config file %v: %v", cfg.Chain.ConfigPath, err)
		}
		cfg.Chain.Config = *chainConfig
		// err = prysmParams.LoadChainConfigFile(cfg.Chain.ConfigPath, nil)
		// if err != nil {
		// 	return fmt.Errorf("error loading chainConfig (%v) for prysmParams: %w", cfg.Chain.ConfigPath, err)
		// }
	}
	cfg.Chain.Name = cfg.Chain.Config.ConfigName

	if cfg.Chain.GenesisTimestamp == 0 {
		switch cfg.Chain.Name {
		case "mainnet":
			cfg.Chain.GenesisTimestamp = 1606824023
		case "prater":
			cfg.Chain.GenesisTimestamp = 1616508000
		case "sepolia":
			cfg.Chain.GenesisTimestamp = 1655733600
		case "zhejiang":
			cfg.Chain.GenesisTimestamp = 1675263600
		case "gnosis":
			cfg.Chain.GenesisTimestamp = 1638993340
		default:
			return fmt.Errorf("tried to set known genesis-timestamp, but unknown chain-name")
		}
	}

	if cfg.Chain.GenesisValidatorsRoot == "" {
		switch cfg.Chain.Name {
		case "mainnet":
			cfg.Chain.GenesisValidatorsRoot = "0x4b363db94e286120d76eb905340fdd4e54bfe9f06bf33ff6cf5ad27f511bfe95"
		case "prater":
			cfg.Chain.GenesisValidatorsRoot = "0x043db0d9a83813551ee2f33450d23797757d430911a9320530ad8a0eabc43efb"
		case "sepolia":
			cfg.Chain.GenesisValidatorsRoot = "0xd8ea171f3c94aea21ebc42a1ed61052acf3f9209c00e4efbaaddac09ed9b8078"
		case "zhejiang":
			cfg.Chain.GenesisValidatorsRoot = "0x53a92d8f2bb1d85f62d16a156e6ebcd1bcaba652d0900b2c2f387826f3481f6f"
		case "gnosis":
			cfg.Chain.GenesisValidatorsRoot = "0xf5dcb5564e829aab27264b9becd5dfaa017085611224cb3036f573368dbb9d47"
		default:
			return fmt.Errorf("tried to set known genesis-validators-root, but unknown chain-name")
		}
	}

	if cfg.Chain.DomainBLSToExecutionChange == "" {
		cfg.Chain.DomainBLSToExecutionChange = "0x0A000000"
	}
	if cfg.Chain.DomainVoluntaryExit == "" {
		cfg.Chain.DomainVoluntaryExit = "0x04000000"
	}

	logrus.WithFields(logrus.Fields{
		"genesisTimestamp":       cfg.Chain.GenesisTimestamp,
		"genesisValidatorsRoot":  cfg.Chain.GenesisValidatorsRoot,
		"configName":             cfg.Chain.Config.ConfigName,
		"depositChainID":         cfg.Chain.Config.DepositChainID,
		"depositNetworkID":       cfg.Chain.Config.DepositNetworkID,
		"depositContractAddress": cfg.Chain.Config.DepositContractAddress,
	}).Infof("did init config")

	return nil
}

func readConfigFile(cfg *types.Config, path string) error {
	if path == "" {
		return yaml.Unmarshal([]byte(config.DefaultConfigYml), cfg)
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("error opening config file %v: %v", path, err)
	}

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(cfg)
	if err != nil {
		return fmt.Errorf("error decoding config file %v: %v", path, err)
	}

	return nil
}

func BitAtVector(b []byte, i int) bool {
	bb := b[i/8]
	return (bb & (1 << uint(i%8))) > 0
}

func ReverseSlice[S ~[]E, E any](s S) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// EpochToTime will return a time.Time for an epoch
func EpochToTime(epoch uint64) time.Time {
	return time.Unix(int64(Config.Chain.GenesisTimestamp+epoch*Config.Chain.Config.SecondsPerSlot*Config.Chain.Config.SlotsPerEpoch), 0)
}

func ToDoc(v interface{}) (doc *bson.D, err error) {
	data, err := bson.Marshal(v)
	if err != nil {
		return nil, err
	}

	err = bson.Unmarshal(data, &doc)
	return
}

func AddBigInts(a, b []byte) []byte {
	return new(big.Int).Add(new(big.Int).SetBytes(a), new(big.Int).SetBytes(b)).Bytes()
}

// LogFatal logs a fatal error with callstack info that skips callerSkip many levels with arbitrarily many additional infos.
// callerSkip equal to 0 gives you info directly where LogFatal is called.
func LogFatal(err error, errorMsg interface{}, callerSkip int, additionalInfos ...string) {
	logErrorInfo(err, callerSkip, additionalInfos...).Fatal(errorMsg)
}

// LogError logs an error with callstack info that skips callerSkip many levels with arbitrarily many additional infos.
// callerSkip equal to 0 gives you info directly where LogError is called.
func LogError(err error, errorMsg interface{}, callerSkip int, additionalInfos ...string) {
	logErrorInfo(err, callerSkip, additionalInfos...).Error(errorMsg)
}

func logErrorInfo(err error, callerSkip int, additionalInfos ...string) *logrus.Entry {
	logFields := logrus.NewEntry(logrus.New())

	pc, fullFilePath, line, ok := runtime.Caller(callerSkip + 2)
	if ok {
		logFields = logFields.WithFields(logrus.Fields{
			"cs_file":     filepath.Base(fullFilePath),
			"cs_function": runtime.FuncForPC(pc).Name(),
			"cs_line":     line,
		})
	} else {
		logFields = logFields.WithField("runtime", "Callstack cannot be read")
	}

	if err != nil {
		logFields = logFields.WithField("error type", fmt.Sprintf("%T", err)).WithError(err)
	}

	for idx, info := range additionalInfos {
		logFields = logFields.WithField(fmt.Sprintf("info_%v", idx), info)
	}

	return logFields
}

// IsEth1Address verifies whether a string represents an eth1-address. In contrast to IsValidEth1Address, this also returns true for the 0x0 address
func IsEth1Address(s string) bool {
	return eth1AddressRE.MatchString(s)
}

func ExchangeRateForCurrency(currency string) float64 {
	return price.GetEthPrice(currency)
}

func SqlRowsToJSON(rows *sql.Rows) ([]interface{}, error) {
	columnTypes, err := rows.ColumnTypes()

	if err != nil {
		return nil, fmt.Errorf("error getting column types: %w", err)
	}

	count := len(columnTypes)
	finalRows := []interface{}{}

	for rows.Next() {

		scanArgs := make([]interface{}, count)

		for i, v := range columnTypes {
			switch v.DatabaseTypeName() {
			case "VARCHAR", "TEXT", "UUID":
				scanArgs[i] = new(sql.NullString)
			case "BOOL":
				scanArgs[i] = new(sql.NullBool)
			case "INT4", "INT8":
				scanArgs[i] = new(sql.NullInt64)
			case "FLOAT8":
				scanArgs[i] = new(sql.NullFloat64)
			case "TIMESTAMP":
				scanArgs[i] = new(sql.NullTime)
			case "_INT4", "_INT8":
				scanArgs[i] = new(pq.Int64Array)
			default:
				scanArgs[i] = new(sql.NullString)
			}
		}

		err := rows.Scan(scanArgs...)

		if err != nil {
			return nil, fmt.Errorf("error scanning rows: %w", err)
		}

		masterData := map[string]interface{}{}

		for i, v := range columnTypes {

			//log.Println(v.Name(), v.DatabaseTypeName())
			if z, ok := (scanArgs[i]).(*sql.NullBool); ok {
				if z.Valid {
					masterData[v.Name()] = z.Bool
				} else {
					masterData[v.Name()] = nil
				}
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullString); ok {
				if z.Valid {
					if v.DatabaseTypeName() == "BYTEA" {
						if len(z.String) > 0 {
							masterData[v.Name()] = "0x" + hex.EncodeToString([]byte(z.String))
						} else {
							masterData[v.Name()] = nil
						}
					} else if v.DatabaseTypeName() == "NUMERIC" {
						nbr, _ := new(big.Int).SetString(z.String, 10)
						masterData[v.Name()] = nbr
					} else {
						masterData[v.Name()] = z.String
					}
				} else {
					masterData[v.Name()] = nil
				}
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullInt64); ok {
				if z.Valid {
					masterData[v.Name()] = z.Int64
				} else {
					masterData[v.Name()] = nil
				}
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullInt32); ok {
				if z.Valid {
					masterData[v.Name()] = z.Int32
				} else {
					masterData[v.Name()] = nil
				}
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullFloat64); ok {
				if z.Valid {
					masterData[v.Name()] = z.Float64
				} else {
					masterData[v.Name()] = nil
				}
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullTime); ok {
				if z.Valid {
					masterData[v.Name()] = z.Time.Unix()
				} else {
					masterData[v.Name()] = nil
				}
				continue
			}

			masterData[v.Name()] = scanArgs[i]
		}

		finalRows = append(finalRows, masterData)
	}

	return finalRows, nil
}

func FormatTokenSymbolTitle(symbol string) string {
	urls := xurls.Relaxed.FindAllString(symbol, -1)

	if len(urls) > 0 {
		return "The token symbol has been hidden as it contains a URL which might be a scam"
	}
	return ""
}

func FormatTokenSymbol(symbol string) string {
	urls := xurls.Relaxed.FindAllString(symbol, -1)

	if len(urls) > 0 {
		return "[hidden-symbol]"
	}
	return symbol
}

func FormatThousandsEnglish(number string) string {
	runes := []rune(number)
	cnt := 0
	for _, rune := range runes {
		if rune == '.' {
			break
		}
		cnt += 1
	}
	amt := cnt / 3
	rem := cnt % 3

	if rem == 0 {
		amt -= 1
	}

	res := make([]rune, 0, amt+rem)
	if amt <= 0 {
		return number
	}
	for i := 0; i < len(runes); i++ {
		if i != 0 && i == rem {
			res = append(res, ',')
			amt -= 1
		}

		if amt > 0 && i > rem && ((i-rem)%3) == 0 {
			res = append(res, ',')
			amt -= 1
		}

		res = append(res, runes[i])
	}

	return string(res)
}

func getABIFromEtherscan(address []byte) (*types.ContractMetadata, error) {
	baseUrl := "api.etherscan.io"
	if Config.Chain.Config.DepositChainID == 5 {
		baseUrl = "api-goerli.etherscan.io"
	}

	httpClient := http.Client{Timeout: time.Second * 5}
	resp, err := httpClient.Get(fmt.Sprintf("https://%s/api?module=contract&action=getsourcecode&address=0x%x&apikey=%s", baseUrl, address, Config.EtherscanAPIKey))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("StatusCode: '%d', Status: '%s'", resp.StatusCode, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	headerData := &struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}{}
	err = json.Unmarshal(body, headerData)
	if err != nil {
		return nil, err
	}
	if headerData.Status == "0" {
		if headerData.Message == "NOTOK" {
			return nil, ErrRateLimit
		}
		return nil, fmt.Errorf("%s", headerData.Message)
	}

	data := &types.EtherscanContractMetadata{}
	err = json.Unmarshal(body, data)
	if err != nil {
		return nil, err
	}
	if data.Result[0].Abi == "Contract source code not verified" {
		return nil, nil
	}

	contractAbi, err := abi.JSON(strings.NewReader(data.Result[0].Abi))
	if err != nil {
		return nil, err
	}
	meta := &types.ContractMetadata{}
	meta.ABIJson = []byte(data.Result[0].Abi)
	meta.ABI = &contractAbi
	meta.Name = data.Result[0].ContractName
	return meta, nil
}

func TryFetchContractMetadata(address []byte) (*types.ContractMetadata, error) {
	return getABIFromEtherscan(address)
}

// SlotToTime returns a time.Time to slot
func SlotToTime(slot uint64) time.Time {
	return time.Unix(int64(Config.Chain.GenesisTimestamp+slot*Config.Chain.Config.SecondsPerSlot), 0)
}

// TimeToEpoch will return an epoch for a given time
func TimeToEpoch(ts time.Time) int64 {
	if int64(Config.Chain.GenesisTimestamp) > ts.Unix() {
		return 0
	}
	return (ts.Unix() - int64(Config.Chain.GenesisTimestamp)) / int64(Config.Chain.Config.SecondsPerSlot) / int64(Config.Chain.Config.SlotsPerEpoch)
}

// EpochOfSlot returns the corresponding epoch of a slot
func EpochOfSlot(slot uint64) uint64 {
	return slot / Config.Chain.Config.SlotsPerEpoch
}

// DayOfSlot returns the corresponding day of a slot
func DayOfSlot(slot uint64) uint64 {
	if slot == 0 {
		return 1
	}
	return Config.Chain.Config.SecondsPerSlot * slot / (24 * 3600)
}

// TimeToDay will return a days since genesis for an timestamp
func TimeToDay(timestamp uint64) uint64 {
	return uint64(time.Unix(int64(timestamp), 0).Sub(time.Unix(int64(Config.Chain.GenesisTimestamp), 0)).Hours() / 24)
	// return time.Unix(int64(Config.Chain.GenesisTimestamp), 0).Add(time.Hour * time.Duration(24*int(day)))
}

func DayToTime(day int64) time.Time {
	return time.Unix(int64(Config.Chain.GenesisTimestamp), 0).Add(time.Hour * time.Duration(24*int(day)))
}

func EpochsPerDay() uint64 {
	day := time.Hour * 24
	return (uint64(day.Seconds()) / Config.Chain.Config.SlotsPerEpoch) / Config.Chain.Config.SecondsPerSlot
}

// TimeToSlot returns time to slot in seconds
func TimeToSlot(timestamp uint64) uint64 {
	if Config.Chain.GenesisTimestamp > timestamp {
		return 0
	}
	return (timestamp - Config.Chain.GenesisTimestamp) / Config.Chain.Config.SecondsPerSlot
}

func SyncPeriodOfEpoch(epoch uint64) uint64 {
	if epoch < Config.Chain.Config.AltairForkEpoch {
		return 0
	}
	return epoch / Config.Chain.Config.EpochsPerSyncCommitteePeriod
}

func FirstEpochOfSyncPeriod(syncPeriod uint64) uint64 {
	return syncPeriod * Config.Chain.Config.EpochsPerSyncCommitteePeriod
}

func GetSigningDomain() ([]byte, error) {
	beaconConfig := prysm_params.BeaconConfig()
	genForkVersion, err := hex.DecodeString(strings.Replace(Config.Chain.Config.GenesisForkVersion, "0x", "", -1))
	if err != nil {
		return nil, err
	}

	domain, err := signing.ComputeDomain(
		beaconConfig.DomainDeposit,
		genForkVersion,
		beaconConfig.ZeroHash[:],
	)

	if err != nil {
		return nil, err
	}

	return domain, err
}

// WaitForCtrlC will block/wait until a control-c is pressed
func WaitForCtrlC() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}

func GraffitiToSring(graffiti []byte) string {
	s := strings.Map(fixUtf, string(bytes.Trim(graffiti, "\x00")))
	s = strings.Replace(s, "\u0000", "", -1) // rempove 0x00 bytes as it is not supported in postgres

	if !utf8.ValidString(s) {
		return "INVALID_UTF8_STRING"
	}

	return s
}

// AddressToWithdrawalCredentials converts a valid address to withdrawalCredentials
func AddressToWithdrawalCredentials(address []byte) ([]byte, error) {
	if IsValidEth1Address(fmt.Sprintf("%#x", address)) {
		credentials := make([]byte, 12, 32)
		credentials[0] = 0x01
		credentials = append(credentials, address...)
		return credentials, nil
	}
	return nil, fmt.Errorf("invalid eth1 address")
}

func fixUtf(r rune) rune {
	if r == utf8.RuneError {
		return -1
	}
	return r
}

// MustParseHex will parse a string into hex
func MustParseHex(hexString string) []byte {
	data, err := hex.DecodeString(strings.Replace(hexString, "0x", "", -1))
	if err != nil {
		log.Fatal(err)
	}
	return data
}

var withdrawalCredentialsRE = regexp.MustCompile("^(0x)?00[0-9a-fA-F]{62}$")
var withdrawalCredentialsAddressRE = regexp.MustCompile("^(0x)?010000000000000000000000[0-9a-fA-F]{40}$")
var eth1TxRE = regexp.MustCompile("^(0x)?[0-9a-fA-F]{64}$")
var zeroHashRE = regexp.MustCompile("^(0x)?0+$")

// IsValidEth1Address verifies whether a string represents a valid eth1-address.
func IsValidEth1Address(s string) bool {
	return !zeroHashRE.MatchString(s) && eth1AddressRE.MatchString(s)
}

// IsValidEth1Tx verifies whether a string represents a valid eth1-tx-hash.
func IsValidEth1Tx(s string) bool {
	return !zeroHashRE.MatchString(s) && eth1TxRE.MatchString(s)
}

// IsValidWithdrawalCredentials verifies whether a string represents valid withdrawal credentials.
func IsValidWithdrawalCredentials(s string) bool {
	return withdrawalCredentialsRE.MatchString(s) || withdrawalCredentialsAddressRE.MatchString(s)
}

// https://github.com/badoux/checkmail/blob/f9f80cb795fa/checkmail.go#L37
var emailRE = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

// IsValidEmail verifies whether a string represents a valid email-address.
func IsValidEmail(s string) bool {
	return emailRE.MatchString(s)
}
