package db_test

import (
	"crypto/rand"
	"database/sql"
	"os"
	"reflect"

	_ "github.com/mattn/go-sqlite3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/renproject/darknode/abi"
	. "github.com/renproject/lightnode/db"
)

var _ = Describe("Lightnode db", func() {
	initDB := func() DB {
		sqlDB, err := sql.Open("sqlite3", "./test.db")
		if err != nil {
			panic(err)
		}
		return NewSQLDB(sqlDB)
	}

	AfterSuite(func() {
		Expect(os.Remove("./test.db")).Should(BeNil())
	})
	AfterEach(func() {
		db := initDB()
		Expect(db.DropGatewayTable()).Should(BeNil())
		Expect(db.CreateGatewayTable()).Should(BeNil())
	})
	Context("When processing GHashes", func() {
		It("Should create the Gateway table", func() {
			db := initDB()
			Expect(db.CreateGatewayTable()).Should(BeNil())
		})

		It("Should be able to insert and retrieve a GHash", func() {
			db := initDB()
			utxo := abi.ExtBtcCompatUTXO{ScriptPubKey: abi.B{}}
			rand.Read(utxo.TxHash[:])
			rand.Read(utxo.GHash[:])

			Expect(db.InsertGateway(utxo)).Should(BeNil())
			utxos, err := db.SelectGateways()
			Expect(err).Should(BeNil())
			Expect(len(utxos)).Should(Equal(1))
			Expect(reflect.DeepEqual(utxos[0], utxo)).Should(BeTrue())
		})

		It("Should be able to update an existing GHash", func() {
			db := initDB()
			utxo1 := abi.ExtBtcCompatUTXO{ScriptPubKey: abi.B{}}
			rand.Read(utxo1.TxHash[:])
			rand.Read(utxo1.GHash[:])

			utxo2 := utxo1
			rand.Read(utxo2.TxHash[:])

			Expect(db.InsertGateway(utxo1)).Should(BeNil())
			Expect(db.InsertGateway(utxo2)).Should(BeNil())
			utxos, err := db.SelectGateways()
			Expect(err).Should(BeNil())
			Expect(len(utxos)).Should(Equal(1))
			Expect(reflect.DeepEqual(utxos[0], utxo2)).Should(BeTrue())
		})

		It("Should be able to insert and retrieve multiple GHashes", func() {
			db := initDB()
			iutxos := make(abi.ExtBtcCompatUTXOs, 5)
			for i := 0; i < 5; i++ {
				utxo := abi.ExtBtcCompatUTXO{ScriptPubKey: abi.B{}}
				rand.Read(utxo.TxHash[:])
				rand.Read(utxo.GHash[:])
				Expect(db.InsertGateway(utxo)).Should(BeNil())
				iutxos[i] = utxo
			}
			utxos, err := db.SelectGateways()
			Expect(err).Should(BeNil())
			Expect(len(utxos)).Should(Equal(5))
			for i := 0; i < 5; i++ {
				Expect(reflect.DeepEqual(utxos[i], iutxos[i])).Should(BeTrue())
			}
		})

		It("Should be able to insert and delete a GHash", func() {
			db := initDB()
			utxo := abi.ExtBtcCompatUTXO{ScriptPubKey: abi.B{}}
			rand.Read(utxo.TxHash[:])
			rand.Read(utxo.GHash[:])

			Expect(db.InsertGateway(utxo)).Should(BeNil())
			Expect(db.DeleteGateway(utxo.GHash)).Should(BeNil())
			utxos, err := db.SelectGateways()
			Expect(err).Should(BeNil())
			Expect(len(utxos)).Should(Equal(0))
		})
	})
})
