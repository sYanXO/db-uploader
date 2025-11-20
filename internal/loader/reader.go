package loader

import (
	"encoding/json"
	"fmt"
	"os"

	"db_uploader/internal/models"
)

// ReadUsers streams users from a JSON file to a channel
func ReadUsers(filePath string, userChan chan<- models.User) error {
	defer close(userChan)

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)

	// Read opening bracket
	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("invalid JSON format, expected opening bracket: %w", err)
	}

	for decoder.More() {
		var user models.User
		if err := decoder.Decode(&user); err != nil {
			return fmt.Errorf("failed to decode user: %w", err)
		}
		userChan <- user
	}

	// Read closing bracket
	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("invalid JSON format, expected closing bracket: %w", err)
	}

	return nil
}
