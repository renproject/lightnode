package cacher

import (
	"github.com/renproject/phi"
	"github.com/sirupsen/logrus"
)

type Cacher struct {
	logger     logrus.FieldLogger
	dispatcher phi.Sender
}

func New(dispatcher phi.Sender, logger logrus.FieldLogger, opts phi.Options) phi.Task {
	return phi.New(&Cacher{logger, dispatcher}, opts)
}

func (cacher *Cacher) Handle(_ phi.Task, message phi.Message) {
	// TODO: Implement caching.
	cacher.dispatcher.Send(message)
}
