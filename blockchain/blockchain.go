package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"github.com/dgraph-io/badger"
	"github.com/pkg/errors"
	"os"
	"runtime"
)

const (
	dbPath = `C:\Projects\Tutorial\Go\golang-blockhain\tmp\blocks`
	dbFile = `C:\Projects\Tutorial\Go\golang-blockhain\tmp\blocks\MANIFEST`
	genesisData = "First transaction from Genesis"
)

type BlockChain struct {
	LastHash []byte
	Database *badger.DB
}

type BlockChainIterator struct {
	CurrentHash []byte
	Database    *badger.DB
}

func DBExist() bool {
	if _,err := os.Stat(dbFile); os.IsNotExist(err) {
		return false;
	}
	return true
}

func InitBlockChain(address string) *BlockChain {
	var lastHash []byte

	if DBExist() {
		fmt.Println("Blockchain already exist")
		runtime.Goexit()
	}

	opts := badger.DefaultOptions
	opts.Dir = dbPath
	opts.ValueDir = dbPath

	db, err := badger.Open(opts)
	Handle(err)

	err = db.Update(func(txn *badger.Txn) error {
		cbtx := CoinBaseTx(address, genesisData)
		genesis := Genesis(cbtx)
		fmt.Println("Genesis is created")
		err = txn.Set(genesis.Hash, genesis.Serialize())
		Handle(err)
		err = txn.Set([]byte("lh"), genesis.Hash)
		lastHash = genesis.Hash

		return err
	})

	Handle(err)

	blockchain := BlockChain{lastHash, db}
	return &blockchain
}

func ContinueBlockChain(address string) *BlockChain{
	if !DBExist() {
		fmt.Println("No existing blockchain. Please create one")
		runtime.Goexit()
	}

	var lastHash []byte

	opts := badger.DefaultOptions
	opts.Dir = dbPath
	opts.ValueDir = dbPath

	db,err := badger.Open(opts)
	Handle(err)

	err = db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		Handle(err)
		lastHash, err = item.Value()
		return err
	})
	Handle(err)
	chain := BlockChain{lastHash, db}
	return &chain
}

func (chain *BlockChain) AddBlock(tranasctions []*Transaction) {
	var lastHash []byte

	err := chain.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		Handle(err)
		lastHash, err = item.Value()

		return err
	})
	Handle(err)

	newBlock := CreateBlock(tranasctions, lastHash)

	err = chain.Database.Update(func(txn *badger.Txn) error {
		err := txn.Set(newBlock.Hash, newBlock.Serialize())
		Handle(err)
		err = txn.Set([]byte("lh"), newBlock.Hash)

		chain.LastHash = newBlock.Hash

		return err
	})
	Handle(err)
}

func (chain *BlockChain) Iterator() *BlockChainIterator {
	iter := &BlockChainIterator{chain.LastHash, chain.Database}

	return iter
}

func (iter *BlockChainIterator) Next() *Block {
	var block *Block

	err := iter.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(iter.CurrentHash)
		Handle(err)
		encodedBlock, err := item.Value()
		block = Deserialize(encodedBlock)

		return err
	})
	Handle(err)

	iter.CurrentHash = block.PrevHash

	return block
}

func (chain *BlockChain) FindUnspentTransactions(pubKeyHash []byte)[]Transaction{
	var unspentTxs []Transaction

	spentTXOs := make(map[string][]int)

	iter := chain.Iterator()
	for {
		block := iter.Next()

		for _,tx := range block.Transactions{
			txID := hex.EncodeToString(tx.ID)
			Outputs:
				for outIdx, out := range tx.Outputs {
					if spentTXOs[txID] != nil {
						for _,spentOut := range spentTXOs[txID]{
							if spentOut == outIdx{
								continue Outputs
							}
						}
					}
					if out.IsLockedWithKey(pubKeyHash) {
						unspentTxs = append(unspentTxs, *tx)
					}
				}
				if tx.IsCoinBase() == false{
					for _,in := range tx.Inputs{
						if in.UsesKey(pubKeyHash){
							inTxId := hex.EncodeToString(in.ID)
							spentTXOs[inTxId] = append(spentTXOs[inTxId], in.Out)
						}
					}
				}
		}
		if len(block.PrevHash) == 0 {
			break
		}
	}
	return unspentTxs
}

func (chain *BlockChain) FindUTXO(pubKeyHash []byte) []TxOutput {
	var UTXOs []TxOutput
	unspentTransactions := chain.FindUnspentTransactions(pubKeyHash)

	for _,tx := range unspentTransactions{
		for _, out := range tx.Outputs{
			if out.IsLockedWithKey(pubKeyHash){
				UTXOs = append(UTXOs, out)
			}
		}
	}
	return UTXOs
}

//this method will enable us to create normal transaction which are not coinbase trx
func (chain *BlockChain) FindSpendableOutputs(pubKeyHash []byte, amount int) (int, map[string][]int){
	unspentOuts := make(map[string][]int)
	unspentTxs := chain.FindUnspentTransactions(pubKeyHash)
	accumulated := 0

	Work:
		for _,tx := range unspentTxs{
			txID := hex.EncodeToString(tx.ID)

			for outIdx, out := range tx.Outputs{
				if out.IsLockedWithKey(pubKeyHash) && accumulated < amount {
					accumulated += out.Value
					unspentOuts[txID] = append(unspentOuts[txID], outIdx)
				}
				if accumulated >= amount{
					break Work
				}
			}
		}

	return accumulated, unspentOuts
}

func (bc *BlockChain) FindTransaction(ID []byte) (Transaction, error){
	iter := bc.Iterator()

	for {
		block := iter.Next()

		for _, tx := range block.Transactions{
			if bytes.Compare(ID, tx.ID) == 0{
				return *tx, nil
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}
	return Transaction{}, errors.New("Transaction does not exist")
}

func (bc *BlockChain) SignTransaction(tx *Transaction, privKey ecdsa.PrivateKey){
	prevTxs := make(map[string]Transaction)

	for _, in:= range tx.Inputs{
		prevTx, err := bc.FindTransaction(in.ID)
		Handle(err)
		prevTxs[hex.EncodeToString(prevTx.ID)] = prevTx
	}
	tx.Sign(privKey, prevTxs)
}

func (bc *BlockChain) VerifyTransaction(tx *Transaction) bool {
	prevTxs := make(map[string]Transaction)

	for _,in := range tx.Inputs {
		prevTx,err := bc.FindTransaction(in.ID)
		Handle(err)
		prevTxs[hex.EncodeToString(prevTx.ID)] = prevTx
	}
	return tx.Verify(prevTxs)
}