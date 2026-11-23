package common

import (
	"embed"
	"errors"
	"fmt"
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	contractutils "github.com/cosmos/evm/contracts/utils"
)

const abiWordLength = 32

// MakeTopic converts a filter query argument into a filter topic.
// NOTE: This was copied from accounts/abi/topics.go
func MakeTopic(rule interface{}) (common.Hash, error) {
	var topic common.Hash

	// Try to generate the topic based on simple types
	switch rule := rule.(type) {
	case common.Hash:
		copy(topic[:], rule[:])
	case common.Address:
		copy(topic[common.HashLength-common.AddressLength:], rule[:])
	case *big.Int:
		blob := rule.Bytes()
		copy(topic[common.HashLength-len(blob):], blob)
	case bool:
		if rule {
			topic[common.HashLength-1] = 1
		}
	case int8:
		copy(topic[:], genIntType(int64(rule), 1))
	case int16:
		copy(topic[:], genIntType(int64(rule), 2))
	case int32:
		copy(topic[:], genIntType(int64(rule), 4))
	case int64:
		copy(topic[:], genIntType(rule, 8))
	case uint8:
		blob := new(big.Int).SetUint64(uint64(rule)).Bytes()
		copy(topic[common.HashLength-len(blob):], blob)
	case uint16:
		blob := new(big.Int).SetUint64(uint64(rule)).Bytes()
		copy(topic[common.HashLength-len(blob):], blob)
	case uint32:
		blob := new(big.Int).SetUint64(uint64(rule)).Bytes()
		copy(topic[common.HashLength-len(blob):], blob)
	case uint64:
		blob := new(big.Int).SetUint64(rule).Bytes()
		copy(topic[common.HashLength-len(blob):], blob)
	case string:
		hash := crypto.Keccak256Hash([]byte(rule))
		copy(topic[:], hash[:])
	case []byte:
		hash := crypto.Keccak256Hash(rule)
		copy(topic[:], hash[:])

	default:
		// todo(rjl493456442) according solidity documentation, indexed event
		// parameters that are not value types i.e. arrays and structs are not
		// stored directly but instead a keccak256-hash of an encoding is stored.
		//
		// We only convert stringS and bytes to hash, still need to deal with
		// array(both fixed-size and dynamic-size) and struct.

		// Attempt to generate the topic from funky types
		val := reflect.ValueOf(rule)
		switch {
		// static byte array
		case val.Kind() == reflect.Array && reflect.TypeOf(rule).Elem().Kind() == reflect.Uint8:
			reflect.Copy(reflect.ValueOf(topic[:val.Len()]), val)
		default:
			return topic, fmt.Errorf("unsupported indexed type: %T", rule)
		}
	}

	return topic, nil
}

// UnpackLog unpacks a retrieved log into the provided output structure.
func UnpackLog(contractABI abi.ABI, out interface{}, event string, log ethtypes.Log) error {
	if log.Topics[0] != contractABI.Events[event].ID {
		return fmt.Errorf("event signature mismatch")
	}
	if len(log.Data) > 0 {
		if err := contractABI.UnpackIntoInterface(out, event, log.Data); err != nil {
			return err
		}
	}
	var indexed abi.Arguments
	for _, arg := range contractABI.Events[event].Inputs {
		if arg.Indexed {
			indexed = append(indexed, arg)
		}
	}
	return abi.ParseTopics(out, indexed, log.Topics[1:])
}

// NOTE: This was copied from accounts/abi/topics.go
func genIntType(rule int64, size uint) []byte {
	var topic [common.HashLength]byte
	if rule < 0 {
		// if a rule is negative, we need to put it into two's complement.
		// extended to common.HashLength bytes.
		topic = [common.HashLength]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}
	}
	for i := uint(0); i < size; i++ {
		topic[common.HashLength-i-1] = byte(rule >> (i * 8))
	}
	return topic[:]
}

// PackNum packs the given number (using the reflect value) and will cast it to appropriate number representation.
func PackNum(value reflect.Value) []byte {
	switch kind := value.Kind(); kind {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return math.U256Bytes(new(big.Int).SetUint64(value.Uint()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return math.U256Bytes(big.NewInt(value.Int()))
	case reflect.Ptr:
		return math.U256Bytes(new(big.Int).Set(value.Interface().(*big.Int)))
	default:
		panic("abi: fatal error")
	}
}

// LoadABI read the ABI file described by the path and parse it as JSON.
func LoadABI(fs embed.FS, path string) (abi.ABI, error) {
	abiBz, err := fs.ReadFile(path)
	if err != nil {
		return abi.ABI{}, fmt.Errorf("error loading the ABI %s", err)
	}

	contract, err := contractutils.ConvertPrecompileHardhatBytesToCompiledContract(abiBz)
	if err != nil {
		return abi.ABI{}, fmt.Errorf(ErrInvalidABI, err)
	}

	return contract.ABI, nil
}

// abiWord reads the 32-byte ABI word at pos and returns it as a position
// within input. It must only be used for offset and length words: values that
// do not fit or that point outside of the input are rejected, since neither
// offsets nor lengths of an in-bounds encoding can exceed the encoding itself.
// Static values (e.g. uint256 amounts) are left to the ABI decoder.
func abiWord(input []byte, pos int) (int, error) {
	if pos < 0 || pos+abiWordLength > len(input) {
		return 0, errors.New("abi: input too short for ABI word")
	}
	v := new(big.Int).SetBytes(input[pos : pos+abiWordLength])
	if !v.IsInt64() || v.Int64() > int64(len(input)) {
		return 0, errors.New("abi: ABI word exceeds input length")
	}
	return int(v.Int64()), nil
}

// abiTypeIsDynamic reports whether the type is encoded behind an offset head
// word instead of being placed inline.
func abiTypeIsDynamic(t abi.Type) bool {
	switch t.T {
	case abi.StringTy, abi.BytesTy, abi.SliceTy:
		return true
	case abi.ArrayTy:
		return abiTypeIsDynamic(*t.Elem)
	case abi.TupleTy:
		for _, elem := range t.TupleElems {
			if abiTypeIsDynamic(*elem) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// abiHeadSize returns the number of bytes the type occupies in the head area
// of its enclosing tuple, array or slice. Dynamic types occupy a single offset
// word, while static arrays and tuples are placed inline with their full
// width. This mirrors the ABI decoder's getTypeSize.
func abiHeadSize(t abi.Type) int {
	if abiTypeIsDynamic(t) {
		return abiWordLength
	}
	switch t.T {
	case abi.ArrayTy:
		return t.Size * abiHeadSize(*t.Elem)
	case abi.TupleTy:
		size := 0
		for _, elem := range t.TupleElems {
			size += abiHeadSize(*elem)
		}
		return size
	default:
		return abiWordLength
	}
}

// abiCharge adds n bytes to the running decode budget. In a canonical
// encoding every charged region (head words, inline static values, length
// words and string/bytes contents) is a disjoint slice of input, so the total
// can never exceed len(input); once it does, the input contains overlapping
// offsets aliasing shared data.
func abiCharge(input []byte, total *int, n int) error {
	if n < 0 || n > len(input)-*total {
		return errors.New("abi: cumulative decoded size exceeds encoded input size")
	}
	*total += n
	return nil
}

// abiElemsCheckSize walks the head area of a tuple (or of the top-level
// arguments) starting at start, whose fields have the given types. The full
// head width is charged before descending into any dynamic field, whose
// offset is relative to start.
func abiElemsCheckSize(elems []abi.Type, start int, input []byte, total *int) error {
	headSize := 0
	for _, elem := range elems {
		headSize += abiHeadSize(elem)
	}
	if err := abiCharge(input, total, headSize); err != nil {
		return err
	}
	fieldStart := start
	for _, elem := range elems {
		if abiTypeIsDynamic(elem) {
			offset, err := abiWord(input, fieldStart)
			if err != nil {
				return err
			}
			if err := abiValueCheckSize(elem, start+offset, input, total); err != nil {
				return err
			}
		}
		fieldStart += abiHeadSize(elem)
	}
	return nil
}

// abiArrayCheckSize charges the heads of size elements of type elem starting
// at start and descends into every dynamic element, whose offset is relative
// to start. Every element is charged at least one word, as the decoder
// reserves one word per element.
func abiArrayCheckSize(elem abi.Type, size, start int, input []byte, total *int) error {
	elemSize := max(abiHeadSize(elem), abiWordLength)
	if size > (len(input)-*total)/elemSize {
		return errors.New("abi: cumulative decoded size exceeds encoded input size")
	}
	if err := abiCharge(input, total, size*elemSize); err != nil {
		return err
	}
	if start+size*elemSize > len(input) {
		return errors.New("abi: array exceeds input length")
	}
	if !abiTypeIsDynamic(elem) {
		// static elements are placed inline and decoded by the ABI decoder
		return nil
	}
	for i := 0; i < size; i++ {
		offset, err := abiWord(input, start+i*abiWordLength)
		if err != nil {
			return err
		}
		if err := abiValueCheckSize(elem, start+offset, input, total); err != nil {
			return err
		}
	}
	return nil
}

// abiValueCheckSize walks the dynamic value of type t whose encoding starts at
// start within input and charges every length word, element head and
// string/bytes content it references to total before descending. Repeated or
// overlapping offsets are charged once per reference, so an input whose
// decoded representation (or decoder traversal) would be larger than the
// encoding is rejected before the decoder materializes it.
func abiValueCheckSize(t abi.Type, start int, input []byte, total *int) error {
	switch t.T {
	case abi.StringTy, abi.BytesTy:
		length, err := abiWord(input, start)
		if err != nil {
			return err
		}
		if start+abiWordLength+length > len(input) {
			return errors.New("abi: dynamic content exceeds input length")
		}
		return abiCharge(input, total, abiWordLength+length)

	case abi.SliceTy:
		size, err := abiWord(input, start)
		if err != nil {
			return err
		}
		if err := abiCharge(input, total, abiWordLength); err != nil {
			return err
		}
		return abiArrayCheckSize(*t.Elem, size, start+abiWordLength, input, total)

	case abi.ArrayTy:
		return abiArrayCheckSize(*t.Elem, t.Size, start, input, total)

	case abi.TupleTy:
		elems := make([]abi.Type, len(t.TupleElems))
		for i, elem := range t.TupleElems {
			elems[i] = *elem
		}
		return abiElemsCheckSize(elems, start, input, total)

	default:
		// static types are placed inline and charged by their parent
		return nil
	}
}

// CheckABIInputSize validates that the ABI-encoded arguments do not expand
// into a decoded representation larger than the encoding itself. The ABI
// decoder validates each dynamic reference independently and accepts
// repeated offsets that all point at the same data, materializing it once
// per reference, so a small encoded input could otherwise unpack into an
// unbounded amount of retained Go memory that is never charged for.
func CheckABIInputSize(args abi.Arguments, input []byte) error {
	total := 0
	elems := make([]abi.Type, len(args))
	for i, arg := range args {
		elems[i] = arg.Type
	}
	return abiElemsCheckSize(elems, 0, input, &total)
}
