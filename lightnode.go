package lightnode

import "github.com/sirupsen/logrus"

type Lightnode struct {
	logger logrus.FieldLogger
}

func New() Lightnode {
	panic("unimplemented")
}

func (lightnode Lightnode) Run() {
}
