package rpc

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
)

// VoteStateLatency represents latency information for a vote
type VoteStateLatency struct {
	Slot              uint64
	ConfirmationCount uint32
	Latency           uint8 // Actual latency from vote state
}

// DeserializeVoteStateLatencies deserializes raw base64 vote account data
// and extracts latency information from Lockout structures
func DeserializeVoteStateLatencies(base64Data string) ([]VoteStateLatency, error) {
	// Decode base64
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	if len(data) < 100 {
		return nil, fmt.Errorf("data too short: %d bytes (expected at least 100)", len(data))
	}

	// VoteState structure (from Solana source code):
	// - VoteStateVersion (u8) - version of the state
	// - authorizedVoters (Vec<AuthorizedVoter>) - Vec: length (u64) + elements
	// - priorVoters (Vec<PriorVoter>) - Vec: length (u64) + elements  
	// - rootSlot (u64)
	// - votes (Vec<Lockout>) - this is what we need! Vec: length (u64) + elements
	// - epochCredits (Vec<EpochCredits>) - skip
	// - lastTimestamp (LastTimestamp) - skip
	// - authorizedWithdrawer (Pubkey) - skip
	// - commission (u8) - skip
	// - etc.

	offset := 0

	// Skip VoteStateVersion (u8)
	if offset >= len(data) {
		return nil, fmt.Errorf("data too short for VoteStateVersion")
	}
	version := data[offset]
	offset++
	
	// Check if version is supported (0, 1, 2 are known versions)
	if version > 2 {
		return nil, fmt.Errorf("unsupported VoteStateVersion: %d (expected 0-2) at offset 0", version)
	}

	// Skip authorizedVoters (Vec<AuthorizedVoter>)
	// AuthorizedVoter: pubkey (32 bytes) + epoch (u64) = 40 bytes
	offset, err = skipVecWithElementSize(data, offset, 40, "authorizedVoters")
	if err != nil {
		return nil, fmt.Errorf("failed to skip authorizedVoters: %w", err)
	}

	// Skip priorVoters (Vec<PriorVoter>)
	// PriorVoter: pubkey (32 bytes) + epoch (u64) = 40 bytes
	offset, err = skipVecWithElementSize(data, offset, 40, "priorVoters")
	if err != nil {
		return nil, fmt.Errorf("failed to skip priorVoters: %w", err)
	}

	// Skip rootSlot (u64)
	if offset+8 > len(data) {
		return nil, fmt.Errorf("data too short for rootSlot at offset %d (data length: %d)", offset, len(data))
	}
	offset += 8

	// Read votes (Vec<Lockout>)
	latencies, err := readLockouts(data, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to read lockouts at offset %d: %w", offset, err)
	}

	return latencies, nil
}

// skipVecWithElementSize skips a Vec<T> in bincode format with known element size
// Vec<T> is: length (u64) + elements
func skipVecWithElementSize(data []byte, offset int, elementSize int, vecName string) (int, error) {
	if offset+8 > len(data) {
		return offset, fmt.Errorf("data too short for %s vec length at offset %d (data length: %d)", 
			vecName, offset, len(data))
	}

	length := binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	// Sanity check: length should be reasonable (max 1000 for authorizedVoters/priorVoters)
	if length > 1000 {
		return offset, fmt.Errorf("invalid %s vec length: %d (too large, possible data corruption or wrong offset at %d)", 
			vecName, length, offset-8)
	}

	if length == 0 {
		// Empty vector, nothing to skip
		return offset, nil
	}

	requiredSize := int(length) * elementSize
	if offset+requiredSize > len(data) {
		return offset, fmt.Errorf("not enough data for %s vec: need %d bytes for %d elements, have %d bytes at offset %d", 
			vecName, requiredSize, length, len(data)-offset, offset)
	}

	offset += requiredSize
	return offset, nil
}

// readLockouts reads Vec<Lockout> from bincode data
// Lockout structure:
// - slot (u64)
// - confirmationCount (u32)
// - latency (u8) - this is what we need!
func readLockouts(data []byte, offset int) ([]VoteStateLatency, error) {
	if offset+8 > len(data) {
		return nil, fmt.Errorf("data too short for votes vec length")
	}

	length := binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	latencies := make([]VoteStateLatency, 0, length)

	// Each Lockout is: slot (u64) + confirmationCount (u32) + latency (u8) = 13 bytes
	lockoutSize := 13

	for i := uint64(0); i < length; i++ {
		if offset+lockoutSize > len(data) {
			// Not enough data - return what we have
			break
		}

		var latency VoteStateLatency
		latency.Slot = binary.LittleEndian.Uint64(data[offset : offset+8])
		offset += 8

		latency.ConfirmationCount = binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4

		latency.Latency = data[offset]
		offset++

		latencies = append(latencies, latency)
	}

	return latencies, nil
}
