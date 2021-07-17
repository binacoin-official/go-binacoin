// Copyright 2015 The go-Binacoin Authors
// This file is part of the go-Binacoin library.
//
// The go-Binacoin library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-Binacoin library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-Binacoin library. If not, see <http://www.gnu.org/licenses/>.

// Contains the metrics collected by the downloader.

package downloader

import (
	"github.com/binacoin-official/go-binacoin/metrics"
)

var (
	headerInMeter      = metrics.NewRegisteredMeter("bna/downloader/headers/in", nil)
	headerReqTimer     = metrics.NewRegisteredTimer("bna/downloader/headers/req", nil)
	headerDropMeter    = metrics.NewRegisteredMeter("bna/downloader/headers/drop", nil)
	headerTimeoutMeter = metrics.NewRegisteredMeter("bna/downloader/headers/timeout", nil)

	bodyInMeter      = metrics.NewRegisteredMeter("bna/downloader/bodies/in", nil)
	bodyReqTimer     = metrics.NewRegisteredTimer("bna/downloader/bodies/req", nil)
	bodyDropMeter    = metrics.NewRegisteredMeter("bna/downloader/bodies/drop", nil)
	bodyTimeoutMeter = metrics.NewRegisteredMeter("bna/downloader/bodies/timeout", nil)

	receiptInMeter      = metrics.NewRegisteredMeter("bna/downloader/receipts/in", nil)
	receiptReqTimer     = metrics.NewRegisteredTimer("bna/downloader/receipts/req", nil)
	receiptDropMeter    = metrics.NewRegisteredMeter("bna/downloader/receipts/drop", nil)
	receiptTimeoutMeter = metrics.NewRegisteredMeter("bna/downloader/receipts/timeout", nil)

	stateInMeter   = metrics.NewRegisteredMeter("bna/downloader/states/in", nil)
	stateDropMeter = metrics.NewRegisteredMeter("bna/downloader/states/drop", nil)

	throttleCounter = metrics.NewRegisteredCounter("bna/downloader/throttle", nil)
)
