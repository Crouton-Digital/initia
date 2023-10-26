package cli

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/initia-labs/initia/x/move/types"
	vmtypes "github.com/initia-labs/initiavm/types"
	"github.com/novifinancial/serde-reflection/serde-generate/runtime/golang/bcs"
	"github.com/novifinancial/serde-reflection/serde-generate/runtime/golang/serde"
	flag "github.com/spf13/pflag"
)

var NewSerializer = bcs.NewSerializer
var NewDeserializer = bcs.NewDeserializer

type argumentDecoder struct {
	// dec is the default decoder
	dec                func(string) ([]byte, error)
	asciiF, hexF, b64F bool
}

func newArgDecoder(def func(string) ([]byte, error)) *argumentDecoder {
	return &argumentDecoder{dec: def}
}

func (a *argumentDecoder) RegisterFlags(f *flag.FlagSet, argName string) {
	f.BoolVar(&a.asciiF, "ascii", false, "ascii encoded "+argName)
	f.BoolVar(&a.hexF, "hex", false, "hex encoded  "+argName)
	f.BoolVar(&a.b64F, "b64", false, "base64 encoded "+argName)
}

func (a *argumentDecoder) DecodeString(s string) ([]byte, error) {
	found := -1
	for i, v := range []*bool{&a.asciiF, &a.hexF, &a.b64F} {
		if !*v {
			continue
		}
		if found != -1 {
			return nil, errors.New("multiple decoding flags used")
		}
		found = i
	}
	switch found {
	case 0:
		return asciiDecodeString(s)
	case 1:
		return hex.DecodeString(s)
	case 2:
		return base64.StdEncoding.DecodeString(s)
	default:
		return a.dec(s)
	}
}

func asciiDecodeString(s string) ([]byte, error) {
	return []byte(s), nil
}

func BcsSerializeArg(argType string, arg string, s serde.Serializer) ([]byte, error) {
	if arg == "" {
		err := s.SerializeBytes([]byte(arg))
		if err != nil {
			return nil, err
		}

		return s.GetBytes(), nil
	}
	switch argType {
	case "raw_hex":
		decoded, err := hex.DecodeString(arg)
		if err != nil {
			return nil, err
		}
		return vmtypes.SerializeBytes(decoded)
	case "raw_base64":
		decoded, err := base64.StdEncoding.DecodeString(arg)
		if err != nil {
			return nil, err
		}
		return vmtypes.SerializeBytes(decoded)
	case "address":
		accAddr, err := types.AccAddressFromString(arg)
		if err != nil {
			return nil, err
		}

		if err := accAddr.Serialize(s); err != nil {
			return nil, err
		}
		return s.GetBytes(), nil

	case "string":
		if err := s.SerializeStr(arg); err != nil {
			return nil, err
		}
		return s.GetBytes(), nil

	case "bool":
		if arg == "true" || arg == "True" {
			_ = s.SerializeBool(true)
		} else if arg == "false" || arg == "False" {
			_ = s.SerializeBool(false)
		} else {
			return nil, errors.New("unsupported bool value")
		}
		return s.GetBytes(), nil

	case "u8", "u16", "u32", "u64":
		bitSize, _ := strconv.Atoi(strings.TrimPrefix(argType, "u"))

		var num uint64
		var err error
		if strings.HasPrefix(arg, "0x") {
			num, err = strconv.ParseUint(strings.TrimPrefix(arg, "0x"), 16, bitSize)
			if err != nil {
				return nil, err
			}
		} else {
			num, err = strconv.ParseUint(arg, 10, bitSize)
			if err != nil {
				return nil, err
			}
		}

		switch argType {
		case "u8":
			_ = s.SerializeU8(uint8(num))
		case "u16":
			_ = s.SerializeU16(uint16(num))
		case "u32":
			_ = s.SerializeU32(uint32(num))
		case "u64":
			_ = s.SerializeU64(num)
		}
		return s.GetBytes(), nil

	case "u128":
		high, low, err := DivideUint128String(arg)
		if err != nil {
			return nil, err
		}
		_ = s.SerializeU128(serde.Uint128{
			Low:  low,
			High: high,
		})
		return s.GetBytes(), err
	case "u256":
		highHigh, highLow, high, low, err := DivideUint256String(arg)
		if err != nil {
			return nil, err
		}
		_ = s.SerializeU128(serde.Uint128{
			Low:  low,
			High: high,
		})
		_ = s.SerializeU128(serde.Uint128{
			Low:  highLow,
			High: highHigh,
		})
		return s.GetBytes(), nil
	default:
		if vectorRegex.MatchString(argType) {
			vecType := getVectorType(argType)
			items := strings.Split(arg, ",")
			if err := s.SerializeLen(uint64(len(items))); err != nil {
				return nil, err
			}
			for _, item := range items {
				_, err := BcsSerializeArg(vecType, item, s)
				if err != nil {
					return nil, err
				}
			}
			return s.GetBytes(), nil
		} else {
			return nil, errors.New("unsupported type arg")
		}
	}
}

var vectorRegex = regexp.MustCompile(`^vector<(.*)>$`)

func getVectorType(vector string) string {
	re := regexp.MustCompile(`<(.*)>`)
	return re.FindStringSubmatch(vector)[1]
}

func DivideUint128String(s string) (uint64, uint64, error) {
	n := new(big.Int)

	var ok bool
	if strings.HasPrefix(s, "0x") {
		_, ok = n.SetString(strings.TrimPrefix(s, "0x"), 16)
	} else {
		_, ok = n.SetString(s, 10)
	}
	if !ok {
		return 0, 0, fmt.Errorf("failed to parse %q as uint128", s)
	}

	if n.Sign() < 0 {
		return 0, 0, errors.New("value cannot be negative")
	} else if n.BitLen() > 128 {
		return 0, 0, errors.New("value overflows Uint128")
	}
	low := n.Uint64()
	high := n.Rsh(n, 64).Uint64()
	return high, low, nil
}

func DivideUint256String(s string) (uint64, uint64, uint64, uint64, error) {
	n := new(big.Int)

	var ok bool
	if strings.HasPrefix(s, "0x") {
		_, ok = n.SetString(strings.TrimPrefix(s, "0x"), 16)
	} else {
		_, ok = n.SetString(s, 10)
	}
	if !ok {
		return 0, 0, 0, 0, fmt.Errorf("failed to parse %q as uint256", s)
	}

	if n.Sign() < 0 {
		return 0, 0, 0, 0, errors.New("value cannot be negative")
	} else if n.BitLen() > 256 {
		return 0, 0, 0, 0, errors.New("value overflows Uint128")
	}
	low := n.Uint64()
	high := n.Rsh(n, 64).Uint64()
	highLow := n.Rsh(n, 64).Uint64()
	highHigh := n.Rsh(n, 64).Uint64()
	return highHigh, highLow, high, low, nil
}