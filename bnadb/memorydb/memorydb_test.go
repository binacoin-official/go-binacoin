// Copyright 2018 The go-binacoin Authors
// This file is part of the go-binacoin library.
//
// The go-binacoin library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-binacoin library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-binacoin library. If not, see <http://www.gnu.org/licenses/>.

package memorydb

import (
	"testing"

	"github.com/binacoin-official/go-binacoin/bnadb"
	"github.com/binacoin-official/go-binacoin/bnadb/dbtest"
)

func TestMemoryDB(t *testing.T) {
	t.Run("DatabaseSuite", func(t *testing.T) {
		dbtest.TestDatabaseSuite(t, func() bnadb.KeyValueStore {
			return New()
		})
	})
}
