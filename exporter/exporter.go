package exporter

import "github.com/sirupsen/logrus"

var logger = logrus.New().WithField("module", "exporter")