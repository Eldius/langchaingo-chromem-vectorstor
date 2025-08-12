package chromem

import (
	"testing"

	"github.com/magiconair/properties/assert"
)

func TestValidateStorageCreation(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		cfg := defaultStorageOptions()

		assert.Equal(t, cfg.dbPath, ".db")
		assert.Equal(t, cfg.collName, "documents")

		WithDBPath("my/new/path")(&cfg)

		assert.Equal(t, cfg.dbPath, "my/new/path")
		assert.Equal(t, cfg.collName, "documents")

		WithCollection("my_collection")(&cfg)
		assert.Equal(t, cfg.collName, "my_collection")
		assert.Equal(t, cfg.dbPath, "my/new/path")
	})
}
