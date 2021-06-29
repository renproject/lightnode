package v0

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// TxStatuses is a wrapper type for the TxStatus slice type.
type TxStatuses []TxStatus

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// TxStatuses type.
func (txStatuses TxStatuses) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(txStatuses))); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write txStatuses len: %v", err)
	}
	for _, txStatus := range txStatuses {
		txStatusData, err := txStatus.MarshalBinary()
		if err != nil {
			return buf.Bytes(), fmt.Errorf("cannot marshal txStatus: %v", err)
		}
		if err := binary.Write(buf, binary.LittleEndian, uint64(len(txStatusData))); err != nil {
			return buf.Bytes(), fmt.Errorf("cannot write txStatus len: %v", err)
		}
		if err := binary.Write(buf, binary.LittleEndian, txStatusData); err != nil {
			return buf.Bytes(), fmt.Errorf("cannot write txStatus data: %v", err)
		}
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryUnmarshaler` interface for the
// TxStatuses type.
func (txStatuses *TxStatuses) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	var lenTxStatuses uint64
	if err := binary.Read(buf, binary.LittleEndian, &lenTxStatuses); err != nil {
		return fmt.Errorf("cannot read txStatuses len: %v", err)
	}
	// TODO: Remove in favour of the surge library. This check restricts the
	// size to be 8192.
	if lenTxStatuses > 8192 {
		return fmt.Errorf("cannot read txStatuses len: too many")
	}
	var numBytes uint64
	txStatusesList := make(TxStatuses, lenTxStatuses)
	for i := uint64(0); i < lenTxStatuses; i++ {
		if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
			return fmt.Errorf("cannot read txStatus len: %v", err)
		}
		// TODO: Remove in favour of the surge library. This check restricts the
		// size to be 1KB.
		if numBytes > 1024 {
			return fmt.Errorf("cannot read txStatus: too many bytes")
		}
		txStatusBytes := make([]byte, numBytes)
		if _, err := buf.Read(txStatusBytes); err != nil {
			return fmt.Errorf("cannot read txStatus data: %v", err)
		}
		if err := txStatusesList[i].UnmarshalBinary(txStatusBytes); err != nil {
			return fmt.Errorf("cannot unmarshal txStatus: %v", err)
		}
	}
	*txStatuses = txStatusesList
	return nil
}

type TxStatus uint8

const (
	TxStatusNil = TxStatus(0)
	// TxStatusConfirming   = TxStatus(1)
	TxStatusPending   = TxStatus(2)
	TxStatusExecuting = TxStatus(3)
	TxStatusDone      = TxStatus(4)
)

func (s TxStatus) String() string {
	switch s {
	case TxStatusNil:
		return "nil"
	case TxStatusPending:
		return "pending"
	case TxStatusExecuting:
		return "executing"
	case TxStatusDone:
		return "done"
	default:
		return ""
	}
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// TxStatus type.
func (s TxStatus) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, s); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write TxStatus: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryUnmarshaler` interface for the
// TxStatus type.
func (s *TxStatus) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	if err := binary.Read(buf, binary.LittleEndian, s); err != nil {
		return fmt.Errorf("cannot read TxStatus: %v", err)
	}
	return nil
}

// Txs is a wrapper type for the Tx slice type.
type Txs []Tx

func (txs Txs) Equal(other Txs) bool {
	if len(txs) != len(other) {
		return false
	}
	for i, tx := range txs {
		if !other[i].Equal(tx) {
			return false
		}
	}
	return true
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the Txs
// type.
func (txs Txs) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(txs))); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write txs len: %v", err)
	}
	for _, tx := range txs {
		txData, err := tx.MarshalBinary()
		if err != nil {
			return buf.Bytes(), fmt.Errorf("cannot marshal tx: %v", err)
		}
		if err := binary.Write(buf, binary.LittleEndian, uint64(len(txData))); err != nil {
			return buf.Bytes(), fmt.Errorf("cannot write tx len: %v", err)
		}
		if err := binary.Write(buf, binary.LittleEndian, txData); err != nil {
			return buf.Bytes(), fmt.Errorf("cannot write tx data: %v", err)
		}
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryUnmarshaler` interface for the
// Txs type.
func (txs *Txs) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	var lenTxs uint64
	if err := binary.Read(buf, binary.LittleEndian, &lenTxs); err != nil {
		return fmt.Errorf("cannot read txs len: %v", err)
	}
	// TODO: Remove in favour of the surge library. This check restricts the
	// size to be 8192.
	if lenTxs > 8192 {
		return fmt.Errorf("cannot read txStatuses len: too many")
	}
	var numBytes uint64
	txsList := make(Txs, lenTxs)
	for i := uint64(0); i < lenTxs; i++ {
		if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
			return fmt.Errorf("cannot read tx len: %v", err)
		}
		// TODO: Remove in favour of the surge library. This check restricts the
		// size to be 10KB.
		if numBytes > 10*1024 {
			return fmt.Errorf("cannot read tx: too many bytes")
		}
		txBytes := make([]byte, numBytes)
		if _, err := buf.Read(txBytes); err != nil {
			return fmt.Errorf("cannot read tx data: %v", err)
		}
		if err := txsList[i].UnmarshalBinary(txBytes); err != nil {
			return fmt.Errorf("cannot unmarshal tx: %v", err)
		}
	}
	*txs = txsList
	return nil
}

// A Tx ABI defines the expected components of a Tx. It must have the Contract
// Address, to which it will be sent, and a slice of Args that it will pass to the
// Contract. The Args must be compatible with the input Formals of the Contract.
type Tx struct {
	Hash    B32     `json:"hash"`
	To      Address `json:"to"`
	In      Args    `json:"in"`
	Autogen Args    `json:"autogen,omitempty"`
	Out     Args    `json:"out,omitempty"`
}

func (tx Tx) Equal(other Tx) bool {
	return tx.Hash.Equal(other.Hash) &&
		tx.To.Equal(other.To) &&
		tx.In.Equal(other.In) &&
		tx.Autogen.Equal(other.Autogen) &&
		tx.Out.Equal(other.Out)
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the Tx
// type.
func (tx Tx) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, tx.Hash); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write tx.Hash: %v", err)
	}
	toBytes := []byte(tx.To)
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(toBytes))); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write tx.To len: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, toBytes); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write tx.To data: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(tx.In))); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write tx.In len: %v", err)
	}
	for _, in := range tx.In {
		inData, err := in.MarshalBinary()
		if err != nil {
			return buf.Bytes(), fmt.Errorf("cannot marshal in: %v", err)
		}
		if err := binary.Write(buf, binary.LittleEndian, uint64(len(inData))); err != nil {
			return buf.Bytes(), fmt.Errorf("cannot write in len: %v", err)
		}
		if err := binary.Write(buf, binary.LittleEndian, inData); err != nil {
			return buf.Bytes(), fmt.Errorf("cannot write in data: %v", err)
		}
	}
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(tx.Autogen))); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write tx.Autogen len: %v", err)
	}
	for _, autogen := range tx.Autogen {
		autogenData, err := autogen.MarshalBinary()
		if err != nil {
			return buf.Bytes(), fmt.Errorf("cannot marshal autogen: %v", err)
		}
		if err := binary.Write(buf, binary.LittleEndian, uint64(len(autogenData))); err != nil {
			return buf.Bytes(), fmt.Errorf("cannot write autogen len: %v", err)
		}
		if err := binary.Write(buf, binary.LittleEndian, autogenData); err != nil {
			return buf.Bytes(), fmt.Errorf("cannot write autogen data: %v", err)
		}
	}
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(tx.Out))); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write tx.Out len: %v", err)
	}
	for _, out := range tx.Out {
		outData, err := out.MarshalBinary()
		if err != nil {
			return buf.Bytes(), fmt.Errorf("cannot marshal out: %v", err)
		}
		if err := binary.Write(buf, binary.LittleEndian, uint64(len(outData))); err != nil {
			return buf.Bytes(), fmt.Errorf("cannot write out len: %v", err)
		}
		if err := binary.Write(buf, binary.LittleEndian, outData); err != nil {
			return buf.Bytes(), fmt.Errorf("cannot write out data: %v", err)
		}
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryUnmarshaler` interface for the
// Tx type.
func (tx *Tx) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	if err := binary.Read(buf, binary.LittleEndian, &tx.Hash); err != nil {
		return fmt.Errorf("cannot read tx.Hash: %v", err)
	}
	var numBytes uint64
	if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
		return fmt.Errorf("cannot read tx.To len: %v", err)
	}
	// TODO: Remove in favour of the surge library. This check restricts the
	// size to be 1KB.
	if numBytes > 1024 {
		return fmt.Errorf("cannot read tx: too many bytes")
	}
	toBytes := make([]byte, numBytes)
	if _, err := buf.Read(toBytes); err != nil {
		return fmt.Errorf("cannot read tx.To data: %v", err)
	}
	tx.To = Address(toBytes)
	var lenIn uint64
	if err := binary.Read(buf, binary.LittleEndian, &lenIn); err != nil {
		return fmt.Errorf("cannot read tx.In len: %v", err)
	}
	// TODO: Remove in favour of the surge library. This check restricts the
	// size to be 64.
	if lenIn > 64 {
		return fmt.Errorf("cannot read tx.In: too many")
	}
	tx.In = make(Args, lenIn)
	for i := uint64(0); i < lenIn; i++ {
		if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
			return fmt.Errorf("cannot read in len: %v", err)
		}
		// TODO: Remove in favour of the surge library. This check restricts the
		// size to be 10KB.
		if numBytes > 10*1024 {
			return fmt.Errorf("cannot read in: too many bytes")
		}
		inBytes := make([]byte, numBytes)
		if _, err := buf.Read(inBytes); err != nil {
			return fmt.Errorf("cannot read in data: %v", err)
		}
		if err := tx.In[i].UnmarshalBinary(inBytes); err != nil {
			return fmt.Errorf("cannot unmarshal in: %v", err)
		}
	}
	var lenAutogen uint64
	if err := binary.Read(buf, binary.LittleEndian, &lenAutogen); err != nil {
		return fmt.Errorf("cannot read tx.Autogen len: %v", err)
	}
	// TODO: Remove in favour of the surge library. This check restricts the
	// size to be 64.
	if lenAutogen > 64 {
		return fmt.Errorf("cannot read tx.Autogen: too many")
	}
	// omitempty
	if lenAutogen > 0 {
		tx.Autogen = make(Args, lenAutogen)
	}
	for i := uint64(0); i < lenAutogen; i++ {
		if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
			return fmt.Errorf("cannot read autogen len: %v", err)
		}
		// TODO: Remove in favour of the surge library. This check restricts the
		// size to be 10KB.
		if numBytes > 10*1024 {
			return fmt.Errorf("cannot read autogen: too many bytes")
		}
		autogenBytes := make([]byte, numBytes)
		if _, err := buf.Read(autogenBytes); err != nil {
			return fmt.Errorf("cannot read autogen data: %v", err)
		}
		if err := tx.Autogen[i].UnmarshalBinary(autogenBytes); err != nil {
			return fmt.Errorf("cannot unmarshal autogen: %v", err)
		}
	}
	var lenOut uint64
	if err := binary.Read(buf, binary.LittleEndian, &lenOut); err != nil {
		return fmt.Errorf("cannot read tx.Out len: %v", err)
	}
	// TODO: Remove in favour of the surge library. This check restricts the
	// size to be 16.
	if lenOut > 16 {
		return fmt.Errorf("cannot read tx.Out: too many")
	}
	// omitempty
	if lenOut > 0 {
		tx.Out = make(Args, lenOut)
	}
	for i := uint64(0); i < lenOut; i++ {
		if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
			return fmt.Errorf("cannot read out len: %v", err)
		}
		// TODO: Remove in favour of the surge library. This check restricts the
		// size to be 1KB.
		if numBytes > 1024 {
			return fmt.Errorf("cannot read out: too many bytes")
		}
		outBytes := make([]byte, numBytes)
		if _, err := buf.Read(outBytes); err != nil {
			return fmt.Errorf("cannot read out data: %v", err)
		}
		if err := tx.Out[i].UnmarshalBinary(outBytes); err != nil {
			return fmt.Errorf("cannot unmarshal out: %v", err)
		}
	}
	return nil
}

// Copy creates a deep copy of the tx object.
func (tx *Tx) Copy() Tx {
	newTx := Tx{
		Hash:    tx.Hash,
		To:      tx.To,
		In:      make([]Arg, len(tx.In)),
		Autogen: make([]Arg, len(tx.Autogen)),
		Out:     make([]Arg, len(tx.Out)),
	}
	if tx.In == nil {
		newTx.In = nil
	}
	if tx.Autogen == nil {
		newTx.Autogen = nil
	}
	if tx.Out == nil {
		newTx.Out = nil
	}

	for i := range newTx.In {
		newTx.In[i] = tx.In[i]
	}
	for i := range newTx.Autogen {
		newTx.Autogen[i] = tx.Autogen[i]
	}
	for i := range newTx.Out {
		newTx.Out[i] = tx.Out[i]
	}
	return newTx
}

// Args is a wrapper type for the Arg slice type.
type Args []Arg

func (args Args) Equal(other Args) bool {
	if len(args) != len(other) {
		return false
	}
	for i, arg := range args {
		if !other[i].Equal(arg) {
			return false
		}
	}
	return true
}

// Get an Argument by its name. This method runs in linear time.
func (args Args) Get(name string) Arg {
	for _, arg := range args {
		if arg.Name == name {
			return arg
		}
	}
	return Arg{}
}

// Set an Argument value by its name. It does not check that the value is of the
// correct type. This method runs in linear time.
func (args *Args) Set(arg Arg) {
	for i := range *args {
		if (*args)[i].Name == arg.Name {
			(*args)[i] = arg
			return
		}
	}
	// The field does not exist, so we append it.
	*args = append(*args, arg)
}

// Remove an Argument by its name. This method runs in linear time.
func (args *Args) Remove(name string) {
	for i, arg := range *args {
		if arg.Name == name {
			*args = append((*args)[:i], (*args)[i+1:]...)
			return
		}
	}
}

// An Arg defines a Value that can fulfil a Formal. It is used to align a Value
// in a Tx to an expected Value in a Contract.
type Arg struct {
	Name  string `json:"name"`
	Type  Type   `json:"type"`
	Value Value  `json:"value"`
}

// IsNil returns true when the Arg is nil, otherwise it returns false.
func (arg *Arg) IsNil() bool {
	return arg.Name == "" && arg.Type == "" && arg.Value == nil
}

func (arg Arg) Equal(other Arg) bool {
	return arg.Name == other.Name &&
		arg.Type == other.Type &&
		arg.Value.Equal(other.Value)
}

// UnmarshalJSON implements json.Unmarshaler for the Arg type.
func (arg *Arg) UnmarshalJSON(data []byte) error {
	raw := struct {
		Name  string          `json:"name"`
		Type  string          `json:"type"`
		Value json.RawMessage `json:"value"`
	}{}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	arg.Name = raw.Name
	arg.Type = Type(raw.Type)

	var err error
	switch raw.Type {

	// Standard types
	case TypeAddress:
		val := Address("")
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val
	case TypeStr:
		val := Str("")
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val
	case TypeB32:
		val := B32{}
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val
	case TypeB:
		val := B{}
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val
	case TypeI8:
		val := I8{}
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val
	case TypeI16:
		val := I16{}
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val
	case TypeI32:
		val := I32{}
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val
	case TypeI64:
		val := I64{}
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val
	case TypeI128:
		val := I128{}
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val
	case TypeI256:
		val := I256{}
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val
	case TypeU8:
		val := U8{}
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val
	case TypeU16:
		val := U16{}
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val
	case TypeU32:
		val := U32{}
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val
	case TypeU64:
		val := U64{}
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val
	case TypeU128:
		val := U128{}
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val
	case TypeU256:
		val := U256{}
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val

	// Extended types
	case ExtTypeEthCompatAddress:
		val := ExtEthCompatAddress{}
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val
	case ExtTypeBtcCompatUTXO:
		val := ExtBtcCompatUTXO{}
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val
	case ExtTypeBtcCompatUTXOs:
		val := ExtBtcCompatUTXOs{}
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val
	case ExtTypeEthCompatTx:
		val := ExtEthCompatTx{}
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val
	case ExtTypeEthCompatPayload:
		val := ExtEthCompatPayload{}
		err = json.Unmarshal(raw.Value, &val)
		arg.Value = val

	default:
		return fmt.Errorf("unexpected type %s", raw.Type)
	}

	return err
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the Arg
// type.
func (arg Arg) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	nameBytes := []byte(arg.Name)
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(nameBytes))); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write arg.Name len: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, nameBytes); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write arg.Name data: %v", err)
	}
	typeBytes := []byte(arg.Type)
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(typeBytes))); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write arg.Type len: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, typeBytes); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write arg.Type data: %v", err)
	}
	valueData, err := arg.Value.MarshalBinary()
	if err != nil {
		return buf.Bytes(), fmt.Errorf("cannot marshal arg.Value: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(valueData))); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write arg.Value len: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, valueData); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write arg.Value data: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryUnmarshaler` interface for the
// Arg type.
func (arg *Arg) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	var numBytes uint64
	if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
		return fmt.Errorf("cannot read arg.Name len: %v", err)
	}
	// TODO: Remove in favour of the surge library. This check restricts the
	// size to be 1KB.
	if numBytes > 1024 {
		return fmt.Errorf("cannot read arg.Name: too many bytes")
	}
	nameBytes := make([]byte, numBytes)
	if _, err := buf.Read(nameBytes); err != nil {
		return fmt.Errorf("cannot read arg.Name data: %v", err)
	}
	arg.Name = string(nameBytes)
	if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
		return fmt.Errorf("cannot read arg.Type len: %v", err)
	}
	// TODO: Remove in favour of the surge library. This check restricts the
	// size to be 1KB.
	if numBytes > 1024 {
		return fmt.Errorf("cannot read arg.Type: too many bytes")
	}
	typeBytes := make([]byte, numBytes)
	if _, err := buf.Read(typeBytes); err != nil {
		return fmt.Errorf("cannot read arg.Type data: %v", err)
	}
	arg.Type = Type(typeBytes)
	if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
		return fmt.Errorf("cannot read arg.Value len: %v", err)
	}
	// TODO: Remove in favour of the surge library. This check restricts the
	// size to be 1MB.
	if numBytes > 1024*1024 {
		return fmt.Errorf("cannot read arg.Value: too many bytes")
	}
	valueBytes := make([]byte, numBytes)
	if _, err := buf.Read(valueBytes); err != nil {
		return fmt.Errorf("cannot read arg.Value data: %v", err)
	}

	var err error
	switch arg.Type {

	// Standard types
	case TypeAddress:
		val := Address("")
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val
	case TypeStr:
		val := Str("")
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val
	case TypeB32:
		val := B32{}
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val
	case TypeB:
		val := B{}
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val
	case TypeI8:
		val := I8{}
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val
	case TypeI16:
		val := I16{}
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val
	case TypeI32:
		val := I32{}
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val
	case TypeI64:
		val := I64{}
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val
	case TypeI128:
		val := I128{}
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val
	case TypeI256:
		val := I256{}
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val
	case TypeU8:
		val := U8{}
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val
	case TypeU16:
		val := U16{}
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val
	case TypeU32:
		val := U32{}
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val
	case TypeU64:
		val := U64{}
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val
	case TypeU128:
		val := U128{}
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val
	case TypeU256:
		val := U256{}
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val

	// Extended types
	case ExtTypeEthCompatAddress:
		val := ExtEthCompatAddress{}
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val
	case ExtTypeBtcCompatUTXO:
		val := ExtBtcCompatUTXO{}
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val
	case ExtTypeBtcCompatUTXOs:
		val := ExtBtcCompatUTXOs{}
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val
	case ExtTypeEthCompatTx:
		val := ExtEthCompatTx{}
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val
	case ExtTypeEthCompatPayload:
		val := ExtEthCompatPayload{}
		err = val.UnmarshalBinary(valueBytes)
		arg.Value = val
	default:
		return fmt.Errorf("unexpected type %s", arg.Type)
	}

	return err
}

// ValidateV0Tx check the tx has a valid contract address and has all the
// required input fields.
func ValidateV0Tx(tx Tx) error {
	// Validate the contract address
	contract, ok := Intrinsics[tx.To]
	if !ok {
		return fmt.Errorf("contract '%v' not found", contract)
	}

	// Check the number of arguments.
	if len(tx.In) < len(contract.In) {
		return fmt.Errorf("%v expects %v arguments, got %v arguments", tx.To, len(contract.In), len(tx.In))
	}

	// Check the tx has all the parameters defined in the contract and each
	// parameter is of the correct type.
	for _, formal := range contract.In {
		arg := tx.In.Get(formal.Name)
		if arg.IsNil() {
			return fmt.Errorf("missing argument [%v] in the tx", formal.Name)
		}
		if arg.Type != formal.Type {
			return fmt.Errorf("%v expects type of [%v] to be '%v', got '%v'", tx.To, formal.Name, formal.Type, arg.Type)
		}
	}
	return nil
}
