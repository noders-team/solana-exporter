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
// Uses pattern matching to find Lockout structures since exact VoteState layout may vary
func DeserializeVoteStateLatencies(base64Data string) ([]VoteStateLatency, error) {
	// Decode base64
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	if len(data) < 100 {
		return nil, fmt.Errorf("data too short: %d bytes (expected at least 100)", len(data))
	}

	// Try to find votes vector by searching for Lockout patterns
	// Lockout: slot (u64) + confirmationCount (u32) + latency (u8) = 13 bytes
	// We'll search for sequences that look like valid Lockout entries
	// A valid slot should be a reasonable number (not too large, not 0)
	// confirmationCount is usually small (0-32)
	// latency is usually 1-10
	
	latencies := findLockoutsByPattern(data)
	if len(latencies) > 0 {
		return latencies, nil
	}

	// Fallback: try structured approach if pattern matching fails
	return deserializeVoteStateStructured(data)
}

// findLockoutsByPattern searches for Lockout structures by pattern matching
// Lockout: slot (u64) + confirmationCount (u32) + latency (u8)
func findLockoutsByPattern(data []byte) []VoteStateLatency {
	var latencies []VoteStateLatency
	lockoutSize := 13
	
	// Search through data looking for valid Lockout patterns
	// Start from offset 100 to skip header/metadata
	for offset := 100; offset+lockoutSize <= len(data); offset++ {
		// Read potential Lockout
		slot := binary.LittleEndian.Uint64(data[offset : offset+8])
		confirmationCount := binary.LittleEndian.Uint32(data[offset+8 : offset+12])
		latency := data[offset+12]
		
		// Validate: slot should be reasonable (between 1 and 2^50, typical Solana slot range)
		// confirmationCount should be small (0-32 typically)
		// latency should be reasonable (1-255, but typically 1-10)
		if slot > 0 && slot < 1<<50 && confirmationCount <= 32 && latency > 0 && latency <= 255 {
			// Check if this looks like a valid sequence (slots should be close to each other)
			// If we already found some, check if this slot is close to previous ones
			if len(latencies) == 0 || 
				(len(latencies) > 0 && slot > latencies[len(latencies)-1].Slot && 
				 slot - latencies[len(latencies)-1].Slot < 1000) {
				latencies = append(latencies, VoteStateLatency{
					Slot:              slot,
					ConfirmationCount: confirmationCount,
					Latency:           latency,
				})
			}
		}
	}
	
	// If we found a reasonable number of votes (typically 32), return them
	if len(latencies) >= 10 && len(latencies) <= 100 {
		return latencies
	}
	
	return nil
}

// deserializeVoteStateStructured attempts structured deserialization (fallback)
func deserializeVoteStateStructured(data []byte) ([]VoteStateLatency, error) {
	// This is the original structured approach - kept as fallback
	offset := 0

	// Skip VoteStateVersion (u8)
	if offset >= len(data) {
		return nil, fmt.Errorf("data too short for VoteStateVersion")
	}
	version := data[offset]
	offset++
	
	if version > 2 {
		return nil, fmt.Errorf("unsupported VoteStateVersion: %d", version)
	}

	// Try to find votes vector by searching for a reasonable length value
	// followed by Lockout-like structures
	for offset < len(data)-100 {
		if offset+8 > len(data) {
			break
		}
		
		potentialLength := binary.LittleEndian.Uint64(data[offset : offset+8])
		
		// Check if this could be a votes vector length (typically 32)
		if potentialLength >= 10 && potentialLength <= 100 {
			// Try to read as votes vector
			latencies, err := readLockouts(data, offset)
			if err == nil && len(latencies) > 0 {
				return latencies, nil
			}
		}
		
		offset++
	}
	
	return nil, fmt.Errorf("could not find votes vector in vote state data")
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
