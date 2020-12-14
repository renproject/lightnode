package v0

import (
	"bytes"
	"encoding"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common/math"
)

// A Type is a constant string that uniquely identifies the type of a Value.
type Type string

// Enumeration of standard types.
const (
	TypeAddress = "address"
	TypeStr     = "str"
	TypeB32     = "b32"
	TypeB       = "b"
	TypeI8      = "i8"
	TypeI16     = "i16"
	TypeI32     = "i32"
	TypeI64     = "i64"
	TypeI128    = "i128"
	TypeI256    = "i256"
	TypeU8      = "u8"
	TypeU16     = "u16"
	TypeU32     = "u32"
	TypeU64     = "u64"
	TypeU128    = "u128"
	TypeU256    = "u256"
	TypeRecord  = "record"
	TypeList    = "list"
)

// A Value is a concrete value associated with a Type.
type Value interface {
	encoding.BinaryMarshaler
	Type() Type
	Equal(Value) bool
}

// Address is a address on RenVM.
type Address string

// Type implements the Value interface for the Address type.
func (Address) Type() Type {
	return TypeAddress
}

// Equal implements the Value interface for the Addr type.
func (address Address) Equal(other Value) bool {
	if other.Type() != TypeAddress {
		return false
	}
	return address == other.(Address)
}

// UnmarshalJSON implements the json.Unmarshaler interface for the Address type.
func (address *Address) UnmarshalJSON(data []byte) error {
	var v string
	err := json.Unmarshal(data, &v)
	*address = Address(v)
	return err
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// Address type.
func (address Address) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	addressBytes := []byte(address)
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(addressBytes))); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write address len: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, addressBytes); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write address data: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryUnmarshaler` interface for the
// Address type.
func (address *Address) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	var numBytes uint64
	if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
		return fmt.Errorf("cannot read address len: %v", err)
	}

	// TODO: Remove in favour of the surge library. This check restricts the
	// size to be 10KB.
	if numBytes > 10*1024 {
		return fmt.Errorf("cannot read address: too many bytes")
	}

	addressBytes := make([]byte, numBytes)
	if _, err := buf.Read(addressBytes); err != nil {
		return fmt.Errorf("cannot read address data: %v", err)
	}
	*address = Address(addressBytes)
	return nil
}

// Str is a dynamically sized string.
type Str string

// Type implements the Value interface for the Str type.
func (Str) Type() Type {
	return TypeStr
}

// Equal implements the Value interface for the Str type.
func (str Str) Equal(other Value) bool {
	if other.Type() != TypeStr {
		return false
	}
	return str == other.(Str)
}

// UnmarshalJSON implements the json.Unmarshaler interface for the Str type.
func (str *Str) UnmarshalJSON(data []byte) error {
	var v string
	err := json.Unmarshal(data, &v)
	*str = Str(v)
	return err
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the Str
// type.
func (str Str) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	strBytes := []byte(str)
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(strBytes))); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write str len: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, strBytes); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write str data: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// Str type.
func (str *Str) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	var numBytes uint64
	if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
		return fmt.Errorf("cannot read str len: %v", err)
	}

	// TODO: Remove in favour of the surge library. This check restricts the
	// size to be 10KB.
	if numBytes > 10*1024 {
		return fmt.Errorf("cannot read str: too many bytes")
	}

	strBytes := make([]byte, numBytes)
	if _, err := buf.Read(strBytes); err != nil {
		return fmt.Errorf("cannot read str data: %v", err)
	}
	*str = Str(strBytes)
	return nil
}

// B32 is a slice of 32 bytes.
type B32 [32]byte

// B32FromHex returns a B32 Value decoded from a hex encoded string. The "0x"
// prefix is optional.
func B32FromHex(str string) (B32, error) {
	if strings.HasPrefix(str, "0x") {
		str = str[2:]
	}
	h, err := hex.DecodeString(str)
	if err != nil {
		return B32([32]byte{}), err
	}
	if len(h) != 32 {
		return B32([32]byte{}), fmt.Errorf("expected %v bytes, got %v byte", 32, len(h))
	}
	b32 := [32]byte{}
	copy(b32[:], h)
	return B32(b32), nil
}

// Type implements the Value interface for the B32 type.
func (B32) Type() Type {
	return TypeB32
}

// Equal implements the Value interface for the B32 type.
func (b32 B32) Equal(other Value) bool {
	if other.Type() != TypeB32 {
		return false
	}
	val := other.(B32)
	return bytes.Equal(b32[:], val[:])
}

// MarshalJSON implements the json.Marshaler interface for the B32 type.
func (b32 B32) MarshalJSON() ([]byte, error) {
	return json.Marshal(b32[:])
}

// UnmarshalJSON implements the json.Unmarshaler interface for the B32 type.
func (b32 *B32) UnmarshalJSON(data []byte) error {
	v := []byte{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	copy(b32[:], v)
	return nil
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the B32
// type.
func (b32 B32) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, b32); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write b32 data: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// B32 type.
func (b32 *B32) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	if err := binary.Read(buf, binary.LittleEndian, b32); err != nil {
		return fmt.Errorf("cannot read b32 data: %v", err)
	}
	return nil
}

// String implements the Stringer interface for the B32 type.
func (b32 B32) String() string {
	return base64.StdEncoding.EncodeToString(b32[:])
}

// B is a slice of bytes.
type B []byte

// BFromHex returns a B Value decoded from a hex encoded string. The "0x"
// prefix is optional.
func BFromHex(str string) (B, error) {
	if strings.HasPrefix(str, "0x") {
		str = str[2:]
	}
	h, err := hex.DecodeString(str)
	if err != nil {
		return B([]byte{}), err
	}
	return B(h), nil
}

// Type implements the Value interface for the B type.
func (B) Type() Type {
	return TypeB
}

// Equal implements the Value interface for the B type.
func (b B) Equal(other Value) bool {
	if other.Type() != TypeB {
		return false
	}
	val := other.(B)
	return bytes.Equal(b[:], val[:])
}

// UnmarshalJSON implements the json.Unmarshaler interface for the B type.
func (b *B) UnmarshalJSON(data []byte) error {
	v := []byte{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*b = v
	return nil
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the B
// type.
func (b B) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(b))); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write b len: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, b); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write b data: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// B type.
func (b *B) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	var numBytes uint64
	if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
		return fmt.Errorf("cannot read b len: %v", err)
	}

	// TODO: Remove in favour of the surge library. This check restricts the
	// size to be 1MB.
	if numBytes > 1024*1024 {
		return fmt.Errorf("cannot read b: too many bytes")
	}

	bBytes := make(B, numBytes)
	if err := binary.Read(buf, binary.LittleEndian, &bBytes); err != nil {
		return fmt.Errorf("cannot read b data: %v", err)
	}
	*b = bBytes
	return nil
}

// String implements the Stringer interface for the B type.
func (b B) String() string {
	return base64.StdEncoding.EncodeToString(b)
}

// I8 stores a big.Int representing a 8 bit unsigned integer.
type I8 struct {
	Int *big.Int
}

// Type implements the Value interface for the I8 type.
func (I8) Type() Type {
	return TypeI8
}

// Equal implements the Value interface for the I8 type.
func (i8 I8) Equal(other Value) bool {
	if other.Type() != TypeI8 {
		return false
	}
	return i8 == other.(I8)
}

// MarshalJSON implements the json.Marshaler interface for the I8 type.
func (i8 I8) MarshalJSON() ([]byte, error) {
	if i8.Int == nil {
		i8.Int = big.NewInt(0)
	}
	return []byte(fmt.Sprintf(`"%s"`, i8.Int.String())), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for the I8 type.
func (i8 *I8) UnmarshalJSON(data []byte) error {
	str := strings.Trim(string(data), "\"")
	if len(str) == 0 {
		i8.Int = big.NewInt(0)
		return nil
	}
	i8Int := new(big.Int)
	_, ok := i8Int.SetString(str, 10)
	if !ok {
		return fmt.Errorf("invalid big integer %s", string(data))
	}
	i8.Int = i8Int
	return nil
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// I8 type.
func (i8 I8) MarshalBinary() ([]byte, error) {
	if i8.Int == nil {
		i8.Int = big.NewInt(0)
	}
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, int8(i8.Int.Int64())); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write i8 data: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// I8 type.
func (i8 *I8) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	var i8Int int8
	if err := binary.Read(buf, binary.LittleEndian, &i8Int); err != nil {
		return fmt.Errorf("cannot read i8 data: %v", err)
	}
	i8.Int = new(big.Int).SetInt64(int64(i8Int))
	return nil
}

// I16 stores a big.Int representing a 16 bit unsigned integer.
type I16 struct {
	Int *big.Int
}

// Type implements the Value interface for the I16 type.
func (I16) Type() Type {
	return TypeI16
}

// Equal implements the Value interface for the I16 type.
func (i16 I16) Equal(other Value) bool {
	if other.Type() != TypeI16 {
		return false
	}
	return i16 == other.(I16)
}

// MarshalJSON implements the json.Marshaler interface for the I16 type.
func (i16 I16) MarshalJSON() ([]byte, error) {
	if i16.Int == nil {
		i16.Int = big.NewInt(0)
	}
	return []byte(fmt.Sprintf(`"%s"`, i16.Int.String())), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for the I16 type.
func (i16 *I16) UnmarshalJSON(data []byte) error {
	str := strings.Trim(string(data), "\"")
	if len(str) == 0 {
		i16.Int = big.NewInt(0)
		return nil
	}
	i16Int := new(big.Int)
	_, ok := i16Int.SetString(str, 10)
	if !ok {
		return fmt.Errorf("invalid big integer %s", string(data))
	}
	i16.Int = i16Int
	return nil
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// I16 type.
func (i16 I16) MarshalBinary() ([]byte, error) {
	if i16.Int == nil {
		i16.Int = big.NewInt(0)
	}
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, int16(i16.Int.Int64())); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write i16 data: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// I16 type.
func (i16 *I16) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	var i16Int int16
	if err := binary.Read(buf, binary.LittleEndian, &i16Int); err != nil {
		return fmt.Errorf("cannot read i16 data: %v", err)
	}
	i16.Int = new(big.Int).SetInt64(int64(i16Int))
	return nil
}

// I32 stores a big.Int representing a 32 bit unsigned integer.
type I32 struct {
	Int *big.Int
}

// Type implements the Value interface for the I32 type.
func (I32) Type() Type {
	return TypeI32
}

// Equal implements the Value interface for the I32 type.
func (i32 I32) Equal(other Value) bool {
	if other.Type() != TypeI32 {
		return false
	}
	return i32 == other.(I32)
}

// MarshalJSON implements the json.Marshaler interface for the I32 type.
func (i32 I32) MarshalJSON() ([]byte, error) {
	if i32.Int == nil {
		i32.Int = big.NewInt(0)
	}
	return []byte(fmt.Sprintf(`"%s"`, i32.Int.String())), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for the I32 type.
func (i32 *I32) UnmarshalJSON(data []byte) error {
	str := strings.Trim(string(data), "\"")
	if len(str) == 0 {
		i32.Int = big.NewInt(0)
		return nil
	}
	i32Int := new(big.Int)
	_, ok := i32Int.SetString(str, 10)
	if !ok {
		return fmt.Errorf("invalid big integer %s", string(data))
	}
	i32.Int = i32Int
	return nil
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// I32 type.
func (i32 I32) MarshalBinary() ([]byte, error) {
	if i32.Int == nil {
		i32.Int = big.NewInt(0)
	}
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, int32(i32.Int.Int64())); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write i32 data: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// I32 type.
func (i32 *I32) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	var i32Int int32
	if err := binary.Read(buf, binary.LittleEndian, &i32Int); err != nil {
		return fmt.Errorf("cannot read i32 data: %v", err)
	}
	i32.Int = new(big.Int).SetInt64(int64(i32Int))
	return nil
}

// I64 stores a big.Int representing a 64 bit unsigned integer.
type I64 struct {
	Int *big.Int
}

// Type implements the Value interface for the I64 type.
func (I64) Type() Type {
	return TypeI64
}

// Equal implements the Value interface for the I64 type.
func (i64 I64) Equal(other Value) bool {
	if other.Type() != TypeI64 {
		return false
	}
	return i64 == other.(I64)
}

// MarshalJSON implements the json.Marshaler interface for the I64 type.
func (i64 I64) MarshalJSON() ([]byte, error) {
	if i64.Int == nil {
		i64.Int = big.NewInt(0)
	}
	return []byte(fmt.Sprintf(`"%s"`, i64.Int.String())), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for the I64 type.
func (i64 *I64) UnmarshalJSON(data []byte) error {
	str := strings.Trim(string(data), "\"")
	if len(str) == 0 {
		i64.Int = big.NewInt(0)
		return nil
	}
	i64Int := new(big.Int)
	_, ok := i64Int.SetString(str, 10)
	if !ok {
		return fmt.Errorf("invalid big integer %s", string(data))
	}
	i64.Int = i64Int
	return nil
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// I64 type.
func (i64 I64) MarshalBinary() ([]byte, error) {
	if i64.Int == nil {
		i64.Int = big.NewInt(0)
	}
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, i64.Int.Int64()); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write i64 data: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// I64 type.
func (i64 *I64) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	var i64Int int64
	if err := binary.Read(buf, binary.LittleEndian, &i64Int); err != nil {
		return fmt.Errorf("cannot read i64 data: %v", err)
	}
	i64.Int = new(big.Int).SetInt64(i64Int)
	return nil
}

// I128 stores a big.Int representing a 128 bit unsigned integer.
type I128 struct {
	Int *big.Int
}

// Type implements the Value interface for the I128 type.
func (I128) Type() Type {
	return TypeI128
}

// Equal implements the Value interface for the I128 type.
func (i128 I128) Equal(other Value) bool {
	if other.Type() != TypeI128 {
		return false
	}
	return i128 == other.(I128)
}

// MarshalJSON implements the json.Marshaler interface for the I128 type.
func (i128 I128) MarshalJSON() ([]byte, error) {
	if i128.Int == nil {
		i128.Int = big.NewInt(0)
	}
	return []byte(fmt.Sprintf(`"%s"`, i128.Int.String())), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for the I128 type.
func (i128 *I128) UnmarshalJSON(data []byte) error {
	str := strings.Trim(string(data), "\"")
	if len(str) == 0 {
		i128.Int = big.NewInt(0)
		return nil
	}
	i128Int := new(big.Int)
	_, ok := i128Int.SetString(str, 10)
	if !ok {
		return fmt.Errorf("invalid big integer %s", string(data))
	}
	i128.Int = i128Int
	return nil
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// I128 type.
func (i128 I128) MarshalBinary() ([]byte, error) {
	if i128.Int == nil {
		i128.Int = big.NewInt(0)
	}
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, int8(i128.Int.Sign())); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write i128 sign: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, math.PaddedBigBytes(i128.Int, 16)); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write i128 bytes: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// I128 type.
func (i128 *I128) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	var i128Sign int8
	if err := binary.Read(buf, binary.LittleEndian, &i128Sign); err != nil {
		return fmt.Errorf("cannot read i128 sign: %v", err)
	}
	i128Bytes := make([]byte, 16)
	if _, err := buf.Read(i128Bytes); err != nil {
		return fmt.Errorf("cannot read i128 bytes: %v", err)
	}
	i128.Int = new(big.Int).SetBytes(i128Bytes)
	if i128Sign < 0 {
		i128.Int = new(big.Int).Mul(i128.Int, big.NewInt(-1))
	}
	return nil
}

// I256 stores a big.Int representing a 256 bit unsigned integer.
type I256 struct {
	Int *big.Int
}

// Type implements the Value interface for the I256 type.
func (I256) Type() Type {
	return TypeI256
}

// Equal implements the Value interface for the I256 type.
func (i256 I256) Equal(other Value) bool {
	if other.Type() != TypeI256 {
		return false
	}
	return i256 == other.(I256)
}

// MarshalJSON implements the json.Marshaler interface for the I256 type.
func (i256 I256) MarshalJSON() ([]byte, error) {
	if i256.Int == nil {
		i256.Int = big.NewInt(0)
	}
	return []byte(fmt.Sprintf(`"%s"`, i256.Int.String())), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for the I256 type.
func (i256 *I256) UnmarshalJSON(data []byte) error {
	str := strings.Trim(string(data), "\"")
	if len(str) == 0 {
		i256.Int = big.NewInt(0)
		return nil
	}
	i256Int := new(big.Int)
	_, ok := i256Int.SetString(str, 10)
	if !ok {
		return fmt.Errorf("invalid big integer %s", string(data))
	}
	i256.Int = i256Int
	return nil
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// I256 type.
func (i256 I256) MarshalBinary() ([]byte, error) {
	if i256.Int == nil {
		i256.Int = big.NewInt(0)
	}
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, int8(i256.Int.Sign())); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write i256 sign: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, math.PaddedBigBytes(i256.Int, 32)); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write i256 bytes: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// I256 type.
func (i256 *I256) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	var i256Sign int8
	if err := binary.Read(buf, binary.LittleEndian, &i256Sign); err != nil {
		return fmt.Errorf("cannot read i256 sign: %v", err)
	}
	i256Bytes := make([]byte, 32)
	if _, err := buf.Read(i256Bytes); err != nil {
		return fmt.Errorf("cannot read i256 bytes: %v", err)
	}
	i256.Int = new(big.Int).SetBytes(i256Bytes)
	if i256Sign < 0 {
		i256.Int = new(big.Int).Mul(i256.Int, big.NewInt(-1))
	}
	return nil
}

// U8 stores a big.Int representing a 8 bit unsigned integer.
type U8 struct {
	Int *big.Int
}

// Type implements the Value interface for the U8 type.
func (U8) Type() Type {
	return TypeU8
}

// Equal implements the Value interface for the U8 type.
func (u8 U8) Equal(other Value) bool {
	if other.Type() != TypeU8 {
		return false
	}
	return u8 == other.(U8)
}

// MarshalJSON implements the json.Marshaler interface for the U8 type.
func (u8 U8) MarshalJSON() ([]byte, error) {
	if u8.Int == nil {
		u8.Int = big.NewInt(0)
	}
	return []byte(fmt.Sprintf(`"%s"`, u8.Int.String())), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for the U8 type.
func (u8 *U8) UnmarshalJSON(data []byte) error {
	str := strings.Trim(string(data), "\"")
	if len(str) == 0 {
		u8.Int = big.NewInt(0)
		return nil
	}
	u8Int := new(big.Int)
	_, ok := u8Int.SetString(str, 10)
	if !ok {
		return fmt.Errorf("invalid big integer %s", string(data))
	}
	if u8Int.Sign() == -1 {
		return fmt.Errorf("expected positive integer")
	}
	u8.Int = u8Int
	return nil
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// U8 type.
func (u8 U8) MarshalBinary() ([]byte, error) {
	if u8.Int == nil {
		u8.Int = big.NewInt(0)
	}
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, int8(u8.Int.Int64())); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write u8 data: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// U8 type.
func (u8 *U8) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	var u8Int uint8
	if err := binary.Read(buf, binary.LittleEndian, &u8Int); err != nil {
		return fmt.Errorf("cannot read u8 data: %v", err)
	}
	u8.Int = new(big.Int).SetUint64(uint64(u8Int))
	return nil
}

// U16 stores a big.Int representing a 16 bit unsigned integer.
type U16 struct {
	Int *big.Int
}

// Type implements the Value interface for the U16 type.
func (U16) Type() Type {
	return TypeU16
}

// Equal implements the Value interface for the U16 type.
func (u16 U16) Equal(other Value) bool {
	if other.Type() != TypeU16 {
		return false
	}
	return u16 == other.(U16)
}

// MarshalJSON implements the json.Marshaler interface for the U16 type.
func (u16 U16) MarshalJSON() ([]byte, error) {
	if u16.Int == nil {
		u16.Int = big.NewInt(0)
	}
	return []byte(fmt.Sprintf(`"%s"`, u16.Int.String())), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for the U16 type.
func (u16 *U16) UnmarshalJSON(data []byte) error {
	str := strings.Trim(string(data), "\"")
	if len(str) == 0 {
		u16.Int = big.NewInt(0)
		return nil
	}
	u16Int := new(big.Int)
	_, ok := u16Int.SetString(str, 10)
	if !ok {
		return fmt.Errorf("invalid big integer %s", string(data))
	}
	if u16Int.Sign() == -1 {
		return fmt.Errorf("expected positive integer")
	}
	u16.Int = u16Int
	return nil
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// U16 type.
func (u16 U16) MarshalBinary() ([]byte, error) {
	if u16.Int == nil {
		u16.Int = big.NewInt(0)
	}
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, int16(u16.Int.Int64())); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write u16 data: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// U16 type.
func (u16 *U16) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	var u16Int uint16
	if err := binary.Read(buf, binary.LittleEndian, &u16Int); err != nil {
		return fmt.Errorf("cannot read u16 data: %v", err)
	}
	u16.Int = new(big.Int).SetUint64(uint64(u16Int))
	return nil
}

// U32 stores a big.Int representing a 32 bit unsigned integer.
type U32 struct {
	Int *big.Int
}

// Type implements the Value interface for the U32 type.
func (U32) Type() Type {
	return TypeU32
}

// Equal implements the Value interface for the U32 type.
func (u32 U32) Equal(other Value) bool {
	if other.Type() != TypeU32 {
		return false
	}
	return u32 == other.(U32)
}

// MarshalJSON implements the json.Marshaler interface for the U32 type.
func (u32 U32) MarshalJSON() ([]byte, error) {
	if u32.Int == nil {
		u32.Int = big.NewInt(0)
	}
	return []byte(fmt.Sprintf(`"%s"`, u32.Int.String())), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for the U32 type.
func (u32 *U32) UnmarshalJSON(data []byte) error {
	str := strings.Trim(string(data), "\"")
	if len(str) == 0 {
		u32.Int = big.NewInt(0)
		return nil
	}
	u32Int := new(big.Int)
	_, ok := u32Int.SetString(str, 10)
	if !ok {
		return fmt.Errorf("invalid big integer %s", string(data))
	}
	if u32Int.Sign() == -1 {
		return fmt.Errorf("expected positive integer")
	}
	u32.Int = u32Int
	return nil
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// U32 type.
func (u32 U32) MarshalBinary() ([]byte, error) {
	if u32.Int == nil {
		u32.Int = big.NewInt(0)
	}
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, int32(u32.Int.Int64())); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write u32 data: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// U32 type.
func (u32 *U32) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	var u32Int uint32
	if err := binary.Read(buf, binary.LittleEndian, &u32Int); err != nil {
		return fmt.Errorf("cannot read u32 data: %v", err)
	}
	u32.Int = new(big.Int).SetUint64(uint64(u32Int))
	return nil
}

// U64 stores a big.Int representing a 64 bit unsigned integer.
type U64 struct {
	Int *big.Int
}

// Type implements the Value interface for the U64 type.
func (U64) Type() Type {
	return TypeU64
}

// Equal implements the Value interface for the U64 type.
func (u64 U64) Equal(other Value) bool {
	if other.Type() != TypeU64 {
		return false
	}
	return u64 == other.(U64)
}

// MarshalJSON implements the json.Marshaler interface for the U64 type.
func (u64 U64) MarshalJSON() ([]byte, error) {
	if u64.Int == nil {
		u64.Int = big.NewInt(0)
	}
	return []byte(fmt.Sprintf(`"%s"`, u64.Int.String())), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for the U64 type.
func (u64 *U64) UnmarshalJSON(data []byte) error {
	str := strings.Trim(string(data), "\"")
	if len(str) == 0 {
		u64.Int = big.NewInt(0)
		return nil
	}
	u64Int := new(big.Int)
	_, ok := u64Int.SetString(str, 10)
	if !ok {
		return fmt.Errorf("invalid big integer %s", string(data))
	}
	if u64Int.Sign() == -1 {
		return fmt.Errorf("expected positive integer")
	}
	u64.Int = u64Int
	return nil
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// U64 type.
func (u64 U64) MarshalBinary() ([]byte, error) {
	if u64.Int == nil {
		u64.Int = big.NewInt(0)
	}
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, int64(u64.Int.Int64())); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write u64 data: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// U64 type.
func (u64 *U64) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	var u64Int uint64
	if err := binary.Read(buf, binary.LittleEndian, &u64Int); err != nil {
		return fmt.Errorf("cannot read u64 data: %v", err)
	}
	u64.Int = new(big.Int).SetUint64(uint64(u64Int))
	return nil
}

// U128 stores a big.Int representing a 128 bit unsigned integer.
type U128 struct {
	Int *big.Int
}

// Type implements the Value interface for the U128 type.
func (U128) Type() Type {
	return TypeU128
}

// Equal implements the Value interface for the U128 type.
func (u128 U128) Equal(other Value) bool {
	if other.Type() != TypeU128 {
		return false
	}
	return u128 == other.(U128)
}

// MarshalJSON implements the json.Marshaler interface for the U128 type.
func (u128 U128) MarshalJSON() ([]byte, error) {
	if u128.Int == nil {
		u128.Int = big.NewInt(0)
	}
	return []byte(fmt.Sprintf(`"%s"`, u128.Int.String())), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for the U128 type.
func (u128 *U128) UnmarshalJSON(data []byte) error {
	str := strings.Trim(string(data), "\"")
	if len(str) == 0 {
		u128.Int = big.NewInt(0)
		return nil
	}
	u128Int := new(big.Int)
	_, ok := u128Int.SetString(str, 10)
	if !ok {
		return fmt.Errorf("invalid big integer %s", string(data))
	}
	if u128Int.Sign() == -1 {
		return fmt.Errorf("expected positive integer")
	}
	u128.Int = u128Int
	return nil
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// U128 type.
func (u128 U128) MarshalBinary() ([]byte, error) {
	if u128.Int == nil {
		u128.Int = big.NewInt(0)
	}
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, math.PaddedBigBytes(u128.Int, 16)); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write u128 bytes: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// U128 type.
func (u128 *U128) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	u128Bytes := make([]byte, 16)
	if _, err := buf.Read(u128Bytes); err != nil {
		return fmt.Errorf("cannot read u128 bytes: %v", err)
	}
	u128.Int = new(big.Int).SetBytes(u128Bytes)
	return nil
}

// U256 stores a big.Int representing a 256 bit unsigned integer.
type U256 struct {
	Int *big.Int
}

// Type implements the Value interface for the U256 type.
func (U256) Type() Type {
	return TypeU256
}

// Equal implements the Value interface for the U256 type.
func (u256 U256) Equal(other Value) bool {
	if other.Type() != TypeU256 {
		return false
	}
	return u256 == other.(U256)
}

// MarshalJSON implements the json.Marshaler interface for the U256 type.
func (u256 U256) MarshalJSON() ([]byte, error) {
	if u256.Int == nil {
		u256.Int = big.NewInt(0)
	}
	return []byte(fmt.Sprintf(`"%s"`, u256.Int.String())), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface for the U256 type.
func (u256 *U256) UnmarshalJSON(data []byte) error {
	str := strings.Trim(string(data), "\"")
	if len(str) == 0 {
		u256.Int = big.NewInt(0)
		return nil
	}
	u256Int := new(big.Int)
	_, ok := u256Int.SetString(str, 10)
	if !ok {
		return fmt.Errorf("invalid big integer %s", string(data))
	}
	if u256Int.Sign() == -1 {
		return fmt.Errorf("expected positive integer")
	}
	u256.Int = u256Int
	return nil
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// U256 type.
func (u256 U256) MarshalBinary() ([]byte, error) {
	if u256.Int == nil {
		u256.Int = big.NewInt(0)
	}
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, math.PaddedBigBytes(u256.Int, 32)); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write u256 bytes: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// U256 type.
func (u256 *U256) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	u256Bytes := make([]byte, 32)
	if _, err := buf.Read(u256Bytes); err != nil {
		return fmt.Errorf("cannot read u256 bytes: %v", err)
	}
	u256.Int = new(big.Int).SetBytes(u256Bytes)
	return nil
}

type Record map[string]Value

// Type implements the Value interface for the Record type.
func (Record) Type() Type {
	return TypeRecord
}

// Equal implements the Value interface for the Record type.
func (record Record) Equal(other Value) bool {
	if other.Type() != TypeRecord {
		return false
	}
	otherRecord := other.(Record)
	if len(record) != len(otherRecord) {
		return false
	}
	for key, val := range record {
		value, ok := otherRecord[key]
		if !ok || !value.Equal(val) {
			return false
		}
	}
	return true
}

// Clone returns a deep clone of the original Record. Modifying this deep clone
// will not modify the original Record.
func (record Record) Clone() Record {
	cloned := Record{}
	for key, val := range record {
		if val, ok := val.(Record); ok {
			cloned[key] = val.Clone()
			continue
		}
		cloned[key] = val
	}
	return cloned
}

// Compatible returns true when the two Records have the field Types. Otherwise,
// it returns false.
func (record Record) Compatible(other Value) bool {
	otherRecord, ok := other.(Record)
	if !ok {
		return false
	}
	if len(record) != len(otherRecord) {
		return false
	}
	for key, val := range record {
		otherVal, ok := otherRecord[key]
		if !ok {
			return false
		}
		if val.Type() != otherVal.Type() {
			return false
		}
		if val.Type() == TypeRecord {
			if !val.(Record).Compatible(otherVal.(Record)) {
				return false
			}
		}
	}
	return true
}

// MarshalJSON implements the `json.Marshaler` interface for the Record
// type.
func (record Record) MarshalJSON() ([]byte, error) {
	args := Args{}
	for name, v := range record {
		args = append(args, Arg{
			Name:  name,
			Type:  v.Type(),
			Value: v,
		})
	}

	sort.Slice(args, func(i, j int) bool {
		return args[i].Name < args[j].Name
	})

	return json.Marshal(args)
}

// UnmarshalJSON implements the `json.Unmarshaler` interface for the
// Record type.
func (record *Record) UnmarshalJSON(data []byte) error {
	args := Args{}
	if err := json.Unmarshal(data, &args); err != nil {
		return err
	}
	values := map[string]Value{}
	for _, arg := range args {
		values[arg.Name] = arg.Value
	}
	*record = Record(values)
	return nil
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// Record type.
func (record Record) MarshalBinary() ([]byte, error) {
	args := Args{}
	for name, v := range record {
		args = append(args, Arg{
			Name:  name,
			Type:  v.Type(),
			Value: v,
		})
	}

	sort.Slice(args, func(i, j int) bool {
		return args[i].Name < args[j].Name
	})

	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(args))); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write args len: %v", err)
	}
	for _, arg := range args {
		argData, err := arg.MarshalBinary()
		if err != nil {
			return buf.Bytes(), fmt.Errorf("cannot marshal arg: %v", err)
		}
		if err := binary.Write(buf, binary.LittleEndian, uint64(len(argData))); err != nil {
			return buf.Bytes(), fmt.Errorf("cannot write arg len: %v", err)
		}
		if err := binary.Write(buf, binary.LittleEndian, argData); err != nil {
			return buf.Bytes(), fmt.Errorf("cannot write arg data: %v", err)
		}
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// Record type.
func (record *Record) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	var lenArgs uint64
	if err := binary.Read(buf, binary.LittleEndian, &lenArgs); err != nil {
		return fmt.Errorf("cannot read args len: %v", err)
	}
	if lenArgs > 50 {
		return fmt.Errorf("cannot allocate memory: too many args (%d)", lenArgs)
	}
	args := make(Args, lenArgs)
	for i := uint64(0); i < lenArgs; i++ {
		var numBytes uint64
		if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
			return fmt.Errorf("cannot read arg len: %v", err)
		}

		// TODO: Remove in favour of the surge library. This check restricts the
		// size to be 1MB.
		if numBytes > 1024*1024 {
			return fmt.Errorf("cannot read arg: too many bytes")
		}

		argBytes := make([]byte, numBytes)
		if _, err := buf.Read(argBytes); err != nil {
			return fmt.Errorf("cannot read arg data: %v", err)
		}
		if err := args[i].UnmarshalBinary(argBytes); err != nil {
			return fmt.Errorf("cannot unmarshal arg: %v", err)
		}
	}

	values := map[string]Value{}
	for _, arg := range args {
		values[arg.Name] = arg.Value
	}
	*record = Record(values)
	return nil
}

type List []Value

func (List) Type() Type {
	return TypeList
}

// Equal implements the Value interface for the List type.
func (list List) Equal(other Value) bool {
	if other.Type() != TypeList {
		return false
	}
	otherList := other.(List)
	if len(list) != len(otherList) {
		return false
	}
	for i, elem := range list {
		if !otherList[i].Equal(elem) {
			return false
		}
	}
	return true
}

func (list List) MarshalBinary() ([]byte, error) {
	panic("unimplemented")
}

func (list *List) UnmarshalBinary(data []byte) error {
	panic("unimplemented")
}

func (list List) MarshalJSON() ([]byte, error) {
	args := Args{}
	for i, v := range list {
		args = append(args, Arg{
			Name:  fmt.Sprintf("%d", i),
			Type:  v.Type(),
			Value: v,
		})
	}
	return json.Marshal(args)
}

func (list *List) UnmarshalJSON(data []byte) error {
	args := Args{}
	if err := json.Unmarshal(data, &args); err != nil {
		return err
	}
	values := make([]Value, len(args))
	for i, arg := range args {
		if arg.Name != fmt.Sprintf("%d", i) {
			return fmt.Errorf("expected %d, got %v", i, arg.Name)
		}
		values = append(values, arg.Value)
	}
	*list = List(values)
	return nil
}
