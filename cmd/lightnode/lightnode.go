package main

import (
	"os"
	"strconv"
	"strings"

	"github.com/renproject/lightnode"
	"github.com/sirupsen/logrus"
)

func main() {
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
	addresses := strings.Split(os.Getenv("ADDRESSES"), ",")

	logger := logrus.New()
	done := make(chan struct{})
	node := lightnode.NewLightnode(logger, cap, workers, timeout, port, addresses)
	node.Run(done)
}
