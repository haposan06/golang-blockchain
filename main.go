package main

import (
	"fmt"
	"github.com/haposan06/golang-blockchain/blockchain"
	"strconv"
)

func main(){
	chain := blockchain.InitBlockChain()
	chain.AddBlock("First Block after Genesis")
	chain.AddBlock("Second block after GENESIS")
	chain.AddBlock("Third block after genesis")

	for _, block := range chain.Blocks{
		fmt.Printf("Previous hash:  %x\n ", block.PrevHash)
		fmt.Printf("Data in block: %s\n", block.Data)
		fmt.Printf("Hash data : %x\n", block.Hash)

		pow := blockchain.NewProof(block);
		fmt.Printf("PoW: %s\n", strconv.FormatBool(pow.Validate()))
		fmt.Println()
	}
}
