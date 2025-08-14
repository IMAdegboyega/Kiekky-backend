package main

import (
    "database/sql"
    "fmt"
    "log"
    "os"
    
    _ "github.com/lib/pq"
    "github.com/joho/godotenv"
)

func main() {
    // Load .env
    err := godotenv.Load()
    if err != nil {
        log.Fatal("Error loading .env file:", err)
    }
    
    fmt.Println("✅ .env loaded successfully!")
    
    // Get DATABASE_URL
    dbURL := os.Getenv("DATABASE_URL")
    if dbURL == "" {
        log.Fatal("DATABASE_URL not found")
    }
    
    fmt.Println("✅ DATABASE_URL found!")
    fmt.Println("Connecting to database...")
    
    // Connect
    db, err := sql.Open("postgres", dbURL)
    if err != nil {
        log.Fatal("Failed to connect:", err)
    }
    defer db.Close()
    
    // Test connection
    if err := db.Ping(); err != nil {
        log.Fatal("Can't reach database:", err)
    }
    
    fmt.Println("✅ Connected to database via .env!")
    
    // Count tables
    var count int
    db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public'").Scan(&count)
    fmt.Printf("✅ Found %d tables\n", count)
}
