package v0

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/renproject/mercury/types/ethtypes"
)

// Enumeration of extended types. Extended types are implemented whenever a new
// distributed ledger is introduced to RenVM, and no standard type can be used
// to represent values needed by the shifter contracts.
const (
	ExtTypeEthCompatAddress = "ext_ethCompatAddress"
	ExtTypeBtcCompatUTXO    = "ext_btcCompatUTXO"
	ExtTypeBtcCompatUTXOs   = "ext_btcCompatUTXOs"
	ExtTypeEthCompatTx      = "ext_ethCompatTx"
	ExtTypeEthCompatPayload = "ext_ethCompatPayload"
)

// ExtEthCompatAddress is an Ethereum compatible address.
type ExtEthCompatAddress ethtypes.Address

// ExtEthCompatAddressFromHex returns a ExtEthCompatAddress value decoded from a
// hex encoded string. The "0x" prefix is optional.
func ExtEthCompatAddressFromHex(str string) (ExtEthCompatAddress, error) {
	if strings.HasPrefix(str, "0x") {
		str = str[2:]
	}
	h, err := hex.DecodeString(str)
	if err != nil {
		return ExtEthCompatAddress{}, err
	}
	if len(h) != 20 {
		return ExtEthCompatAddress{}, fmt.Errorf("expected %v bytes, got %v byte", 20, len(h))
	}
	extEthCompatAddress := [20]byte{}
	copy(extEthCompatAddress[:], h)
	return ExtEthCompatAddress(extEthCompatAddress), nil
}

// Type implements the Value interface for the ExtEthCompatAddress type.
func (ExtEthCompatAddress) Type() Type {
	return ExtTypeEthCompatAddress
}

// Equal implements the Value interface for the B20 type.
func (extEthCompatAddress ExtEthCompatAddress) Equal(other Value) bool {
	if other.Type() != ExtTypeEthCompatAddress {
		return false
	}
	val := other.(ExtEthCompatAddress)
	return bytes.Equal(extEthCompatAddress[:], val[:])
}

// MarshalJSON implements the json.Unmarshaler interface for the
// ExtEthCompatAddress type.
func (extEthCompatAddress ExtEthCompatAddress) MarshalJSON() ([]byte, error) {
	return json.Marshal(extEthCompatAddress.String())
}

// UnmarshalJSON implements the json.Unmarshaler interface for the
// ExtEthCompatAddress type.
func (extEthCompatAddress *ExtEthCompatAddress) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	vBytes, err := hex.DecodeString(v)
	if err != nil {
		return err
	}
	copy(extEthCompatAddress[:], vBytes)
	return nil
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// ExtEthCompatAddress type.
func (extEthCompatAddress ExtEthCompatAddress) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, extEthCompatAddress); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write extEthCompatAddress data: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// ExtEthCompatAddress type.
func (extEthCompatAddress *ExtEthCompatAddress) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	if err := binary.Read(buf, binary.LittleEndian, extEthCompatAddress); err != nil {
		return fmt.Errorf("cannot read extEthCompatAddress data: %v", err)
	}
	return nil
}

// String implements the Stringer interface for the ExtEthCompatAddress type.
func (extEthCompatAddress ExtEthCompatAddress) String() string {
	return hex.EncodeToString(extEthCompatAddress[:])
}

// ExtBtcCompatUTXOs is a wrapper type.
type ExtBtcCompatUTXOs []ExtBtcCompatUTXO

// Equal implements the Value interface for the ExtBtcCompatUTXOs type.
func (utxos ExtBtcCompatUTXOs) Equal(other Value) bool {
	otherUTXOs, ok := other.(ExtBtcCompatUTXOs)
	if !ok {
		return false
	}
	if len(utxos) != len(otherUTXOs) {
		return false
	}
	if len(utxos) == 0 {
		return true
	}
	for i := range utxos {
		if !utxos[i].Equal(otherUTXOs[i]) {
			return false
		}
	}
	return true
}

// Type implements the Value interface for the ExtBtcCompatUTXOs type.
func (ExtBtcCompatUTXOs) Type() Type {
	return ExtTypeBtcCompatUTXOs
}

// Amount returns the sum of all amounts in the UTXOs.
func (extBtcCompatUTXOs ExtBtcCompatUTXOs) Amount() U256 {
	total := big.NewInt(0)
	for _, utxo := range extBtcCompatUTXOs {
		total = new(big.Int).Add(total, utxo.Amount.Int)
	}
	return U256{Int: total}
}

// Sort the UTXOs by amount, from smallest to largest.
func (utxos ExtBtcCompatUTXOs) Sort() {
	sort.SliceStable(utxos, func(i, j int) bool {
		// utxos[i] amount < utxos[j] amount
		return utxos[i].Amount.Int.Cmp(utxos[j].Amount.Int) == -1
	})
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// ExtBtcCompatUTXOs type.
func (extBtcCompatUTXOs ExtBtcCompatUTXOs) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(extBtcCompatUTXOs))); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write extBtcCompatUTXOs len: %v", err)
	}
	for _, extBtcCompatUTXO := range extBtcCompatUTXOs {
		extBtcCompatUTXOData, err := extBtcCompatUTXO.MarshalBinary()
		if err != nil {
			return buf.Bytes(), fmt.Errorf("cannot marshal extBtcCompatUTXO: %v", err)
		}
		if err := binary.Write(buf, binary.LittleEndian, uint64(len(extBtcCompatUTXOData))); err != nil {
			return buf.Bytes(), fmt.Errorf("cannot write extBtcCompatUTXO len: %v", err)
		}
		if err := binary.Write(buf, binary.LittleEndian, extBtcCompatUTXOData); err != nil {
			return buf.Bytes(), fmt.Errorf("cannot write extBtcCompatUTXO data: %v", err)
		}
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// ExtBtcCompatUTXOs type.
func (extBtcCompatUTXOs *ExtBtcCompatUTXOs) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	var lenExtBtcCompatUTXOs uint64
	if err := binary.Read(buf, binary.LittleEndian, &lenExtBtcCompatUTXOs); err != nil {
		return fmt.Errorf("cannot read extBtcCompatUTXOs len: %v", err)
	}

	// TODO: Remove in favour of the surge library. This check restricts the
	// size to be 4096.
	if lenExtBtcCompatUTXOs > 4096 {
		return fmt.Errorf("cannot read extBtcCompatUTXOs: too many")
	}

	extBtcCompatUTXOsList := make(ExtBtcCompatUTXOs, lenExtBtcCompatUTXOs)
	var numBytes uint64
	for i := uint64(0); i < lenExtBtcCompatUTXOs; i++ {
		if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
			return fmt.Errorf("cannot read extBtcCompatUTXO len: %v", err)
		}

		// TODO: Remove in favour of the surge library. This check restricts the
		// size to be 10 KB.
		if numBytes > 10*1024 {
			return fmt.Errorf("cannot read extBtcCompatUTXO: too many bytes")
		}
		extBtcCompatUTXOBytes := make([]byte, numBytes)
		if _, err := buf.Read(extBtcCompatUTXOBytes); err != nil {
			return fmt.Errorf("cannot read extBtcCompatUTXO data: %v", err)
		}
		if err := extBtcCompatUTXOsList[i].UnmarshalBinary(extBtcCompatUTXOBytes); err != nil {
			return fmt.Errorf("cannot unmarshal extBtcCompatUTXO: %v", err)
		}
	}
	*extBtcCompatUTXOs = extBtcCompatUTXOsList
	return nil
}

// ExtBtcCompatUTXO is a Bitcoin compatible UTXO. It is used for transactions on
// Bitcoin (and forks).
type ExtBtcCompatUTXO struct {
	TxHash B32 `json:"txHash"`
	VOut   U32 `json:"vOut"`

	ScriptPubKey B    `json:"scriptPubKey,omitempty"`
	Amount       U256 `json:"amount,omitempty"`
	GHash        B32  `json:"ghash,omitempty"`
}

// Type implements the Value interface for the ExtBtcCompatUTXO type.
func (ExtBtcCompatUTXO) Type() Type {
	return ExtTypeBtcCompatUTXO
}

// Equal implements the Value interface for the ExtBtcCompatUTXO type.
func (utxo ExtBtcCompatUTXO) Equal(other Value) bool {
	otherUTXO, ok := other.(ExtBtcCompatUTXO)
	if !ok {
		return false
	}
	return utxo.TxHash.Equal(otherUTXO.TxHash) &&
		utxo.VOut.Equal(otherUTXO.VOut) &&
		utxo.ScriptPubKey.Equal(otherUTXO.ScriptPubKey) &&
		utxo.Amount.Equal(otherUTXO.Amount) &&
		utxo.GHash.Equal(otherUTXO.GHash)
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// ExtBtcCompatUTXO type.
func (extBtcCompatUTXO ExtBtcCompatUTXO) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, extBtcCompatUTXO.TxHash); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write extBtcCompatUTXO.TxHash data: %v", err)
	}
	vOutBytes, err := extBtcCompatUTXO.VOut.MarshalBinary()
	if err != nil {
		return buf.Bytes(), fmt.Errorf("cannot marshal extBtcCompatUTXO.VOut: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, vOutBytes); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write extBtcCompatUTXO.VOut data: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(extBtcCompatUTXO.ScriptPubKey))); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write extBtcCompatUTXO.ScriptPubKey len: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, extBtcCompatUTXO.ScriptPubKey); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write extBtcCompatUTXO.ScriptPubKey data: %v", err)
	}
	amountBytes, err := extBtcCompatUTXO.Amount.MarshalBinary()
	if err != nil {
		return buf.Bytes(), fmt.Errorf("cannot marshal extBtcCompatUTXO.Amount: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, amountBytes); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write extBtcCompatUTXO.Amount data: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, extBtcCompatUTXO.GHash); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write extBtcCompatUTXO.GHash data: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// ExtBtcCompatUTXO type.
func (extBtcCompatUTXO *ExtBtcCompatUTXO) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	if err := binary.Read(buf, binary.LittleEndian, &extBtcCompatUTXO.TxHash); err != nil {
		return fmt.Errorf("cannot read extBtcCompatUTXO.TxHash data: %v", err)
	}
	vOutBytes := make([]byte, 4)
	if _, err := buf.Read(vOutBytes); err != nil {
		return fmt.Errorf("cannot read extBtcCompatUTXO.VOut data: %v", err)
	}
	if err := extBtcCompatUTXO.VOut.UnmarshalBinary(vOutBytes); err != nil {
		return fmt.Errorf("cannot unmarshal extBtcCompatUTXO.VOut: %v", err)
	}
	var numBytes uint64
	if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
		return fmt.Errorf("cannot read extBtcCompatUTXO.ScriptPubKey len: %v", err)
	}

	// TODO: Remove in favour of the surge library. This check restricts the
	// size to be 1KB.
	if numBytes > 1024 {
		return fmt.Errorf("cannot read extBtcCompatUTXO: too many bytes")
	}
	scriptPubKeyBytes := make([]byte, numBytes)
	if _, err := buf.Read(scriptPubKeyBytes); err != nil {
		return fmt.Errorf("cannot read extBtcCompatUTXO.ScriptPubKey data: %v", err)
	}
	extBtcCompatUTXO.ScriptPubKey = scriptPubKeyBytes
	amountBytes := make([]byte, 32)
	if _, err := buf.Read(amountBytes); err != nil {
		return fmt.Errorf("cannot read extBtcCompatUTXO.Amount data: %v", err)
	}
	if err := extBtcCompatUTXO.Amount.UnmarshalBinary(amountBytes); err != nil {
		return fmt.Errorf("cannot unmarshal extBtcCompatUTXO.Amount: %v", err)
	}
	if err := binary.Read(buf, binary.LittleEndian, &extBtcCompatUTXO.GHash); err != nil {
		return fmt.Errorf("cannot read extBtcCompatUTXO.GHash data: %v", err)
	}
	return nil
}

// ExtEthCompatTx is an Ethereum compatible transaction. It is used to
// representation transactions on Ethereum (and forks).
type ExtEthCompatTx struct {
	TxHash B32 `json:"txHash"`
}

// Type implements the Value interface for the ExtEthCompatTx type.
func (ExtEthCompatTx) Type() Type {
	return ExtTypeEthCompatTx
}

// Equal implements the Value interface for the ExtEthCompatTx type.
func (tx ExtEthCompatTx) Equal(other Value) bool {
	otherTx, ok := other.(ExtEthCompatTx)
	if !ok {
		return false
	}
	return tx.TxHash.Equal(otherTx.TxHash)
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// ExtEthCompatTx type.
func (extEthCompatTx ExtEthCompatTx) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, extEthCompatTx.TxHash); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write extEthCompatTx.TxHash data: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// ExtEthCompatTx type.
func (extEthCompatTx *ExtEthCompatTx) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	if err := binary.Read(buf, binary.LittleEndian, &extEthCompatTx.TxHash); err != nil {
		return fmt.Errorf("cannot read extEthCompatTx.TxHash data: %v", err)
	}
	return nil
}

// ExtEthCompatPayload is an Ethereum compatible packed payload.
type ExtEthCompatPayload struct {
	ABI   B `json:"abi"`
	Value B `json:"value"`
	Fn    B `json:"fn"`
}

// Type implements the Value interface for the ExtEthCompatPayload type.
func (ExtEthCompatPayload) Type() Type {
	return ExtTypeEthCompatPayload
}

// Equal implements the Value interface for the ExtEthCompatPayload type.
func (payload ExtEthCompatPayload) Equal(other Value) bool {
	otherPayload, ok := other.(ExtEthCompatPayload)
	if !ok {
		return false
	}
	return payload.ABI.Equal(otherPayload.ABI) && payload.Value.Equal(otherPayload.Value) && payload.Fn.Equal(otherPayload.Fn)
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// ExtEthCompatPayload type.
func (payload ExtEthCompatPayload) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)

	// Write ABI
	abiBytes, err := payload.ABI.MarshalBinary()
	if err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, abiBytes); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write ExtEthCompatPayload.ABI: %v", err)
	}

	// Write Value
	valueBytes, err := payload.Value.MarshalBinary()
	if err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, valueBytes); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write ExtEthCompatPayload.Value: %v", err)
	}

	// Write Fn
	fnBytes, err := payload.Fn.MarshalBinary()
	if err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, fnBytes); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write ExtEthCompatPayload.Fn: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// ExtEthCompatPayload type.
func (payload *ExtEthCompatPayload) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)

	// Read ABI
	var numBytes uint64
	if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
		return fmt.Errorf("cannot read ExtEthCompatPayload.ABI len: %v", err)
	}

	// TODO: Remove in favour of the surge library. This check restricts the
	// size to be 1MB.
	if numBytes > 1024*1024 {
		return fmt.Errorf("cannot read ExtEthCompayPayload.ABI: too many bytes")
	}
	abiBytes := make(B, numBytes)
	if _, err := buf.Read(abiBytes); err != nil {
		return fmt.Errorf("cannot read ExtEthCompatPayload.ABI data: %v", err)
	}
	payload.ABI = abiBytes

	// Read Value
	if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
		return fmt.Errorf("cannot read ExtEthCompatPayload.Value len: %v", err)
	}
	// TODO: Remove in favour of the surge library. This check restricts the
	// size to be 1MB.
	if numBytes > 1024*1024 {
		return fmt.Errorf("cannot read ExtEthCompayPayload.Value: too many bytes")
	}
	valueBytes := make(B, numBytes)
	if _, err := buf.Read(valueBytes); err != nil {
		return fmt.Errorf("cannot read ExtEthCompatPayload.Value data: %v", err)
	}
	payload.Value = valueBytes

	// Read Fn
	if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
		return fmt.Errorf("cannot read ExtEthCompatPayload.Fn len: %v", err)
	}
	// TODO: Remove in favour of the surge library. This check restricts the
	// size to be 1KB.
	if numBytes > 1024 {
		return fmt.Errorf("cannot read ExtEthCompayPayload.Fn: too many bytes")
	}
	fnBytes := make(B, numBytes)
	if _, err := buf.Read(fnBytes); err != nil {
		return fmt.Errorf("cannot read ExtEthCompatPayload.Fn data: %v", err)
	}
	payload.Fn = fnBytes

	return nil
}

// Hash returns the keccak hash of the payload.
func (payload *ExtEthCompatPayload) Hash() B32 {
	hash := crypto.Keccak256(payload.Value)
	var phash B32
	copy(phash[:], hash)
	return phash
}
