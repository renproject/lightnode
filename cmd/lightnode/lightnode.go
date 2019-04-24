package main

import (
	"os"
	"strconv"

	"github.com/republicprotocol/lightnode"
	"github.com/sirupsen/logrus"
)

func main() {
	// TODO : Are we getting this from ENV or config files?
	port := os.Getenv("PORT")
	cap, err := strconv.Atoi(os.Getenv("CAP"))
	if err != nil {
		cap = 128
	}
	workers, err := strconv.Atoi(os.Getenv("WORKERS"))
	if err != nil {
		workers = 16
	}
	timeout, err := strconv.Atoi(os.Getenv("TIMEOUT"))
	if err != nil {
		timeout = 60
	}

	logger := logrus.New()
	done := make(chan struct{})
	node := lightnode.NewLightNode(logger, cap, workers, timeout, port)
	node.Run(done)
}
