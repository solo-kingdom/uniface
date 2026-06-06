package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// HopIdempotencyKey 生成 hop 幂等键。
func HopIdempotencyKey(entityID, nodeID string, inputSequence int64) string {
	return hashParts(entityID, nodeID, fmt.Sprintf("%d", inputSequence))
}

// CompensationIdempotencyKey 生成补偿幂等键。
func CompensationIdempotencyKey(entityID string, forwardSequence int64, compensatorUnitID string) string {
	return hashParts(entityID, "comp", fmt.Sprintf("%d", forwardSequence), compensatorUnitID)
}

// SignalIdempotencyKey 生成信号投递幂等键。
func SignalIdempotencyKey(entityID, signalName, deliveryID string) string {
	return hashParts(entityID, signalName, deliveryID)
}

func hashParts(parts ...string) string {
	h := sha256.New()
	for i, p := range parts {
		if i > 0 {
			h.Write([]byte{0})
		}
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}
