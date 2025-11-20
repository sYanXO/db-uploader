package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"db_uploader/internal/models"
)

func main() {
	outputFile := flag.String("output", "data.json", "Output file path")
	numRecords := flag.Int("count", 100000, "Number of records to generate")
	flag.Parse()

	file, err := os.Create(*outputFile)
	if err != nil {
		log.Fatalf("Failed to create file: %v", err)
	}
	defer file.Close()

	if _, err := file.WriteString("[\n"); err != nil {
		log.Fatalf("Failed to write opening bracket: %v", err)
	}

	encoder := json.NewEncoder(file)
	rand.Seed(time.Now().UnixNano())

	for i := 0; i < *numRecords; i++ {
		user := models.User{
			ID:        i + 1,
			Name:      fmt.Sprintf("User %d", i+1),
			Email:     fmt.Sprintf("user%d@example.com", i+1),
			Age:       rand.Intn(60) + 18,
			City:      randomCity(),
			CreatedAt: time.Now().Format(time.RFC3339),
		}

		if i > 0 {
			if _, err := file.WriteString(",\n"); err != nil {
				log.Fatalf("Failed to write comma: %v", err)
			}
		}

		if err := encoder.Encode(user); err != nil {
			log.Fatalf("Failed to encode user: %v", err)
		}
	}

	if _, err := file.WriteString("\n]"); err != nil {
		log.Fatalf("Failed to write closing bracket: %v", err)
	}

	fmt.Printf("Successfully generated %d records to %s\n", *numRecords, *outputFile)
}

func randomCity() string {
	cities := []string{"New York", "London", "Tokyo", "Paris", "Berlin", "Sydney", "Mumbai", "Toronto"}
	return cities[rand.Intn(len(cities))]
}
