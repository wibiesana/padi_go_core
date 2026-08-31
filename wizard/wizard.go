package wizard

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wibiesana/padi_go_core/config"
	"github.com/wibiesana/padi_go_core/database"
	"github.com/wibiesana/padi_go_core/migrator"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

type Wizard struct {
	reader  *bufio.Reader
	baseDir string
}

func New(baseDir string) *Wizard {
	if baseDir == "" {
		baseDir = "."
	}
	return &Wizard{
		reader:  bufio.NewReader(os.Stdin),
		baseDir: baseDir,
	}
}

func (w *Wizard) Run() error {
	fmt.Println()
	fmt.Println(colorCyan + colorBold + "==========================================================" + colorReset)
	fmt.Println(colorCyan + colorBold + "   🌾 Welcome to PADI REST API GO - Setup Wizard" + colorReset)
	fmt.Println(colorCyan + colorBold + "==========================================================" + colorReset)
	fmt.Println("This wizard will guide you through setting up your database,")
	fmt.Println("generating your JWT security keys, and creating your .env configuration.")
	fmt.Println()

	// 1. Choose Database Driver
	fmt.Println(colorBold + "👉 Select Database Driver:" + colorReset)
	fmt.Println("  [1] SQLite (Pure Go, Embedded, Zero-Setup - Recommended)")
	fmt.Println("  [2] MySQL / MariaDB")
	fmt.Println("  [3] PostgreSQL")
	dbChoice := w.askChoice("Enter database option [1-3]", 1, 3, 1)

	var (
		dbDriver   string
		dbHost     string
		dbPort     string
		dbName     string
		dbUser     string
		dbPassword string
	)

	switch dbChoice {
	case 1:
		dbDriver = "sqlite"
		dbName = w.ask("SQLite database file path", "database/database.sqlite")
		_ = os.MkdirAll(filepath.Dir(filepath.Join(w.baseDir, dbName)), 0755)
	case 2:
		dbDriver = "mysql"
		dbHost = w.ask("MySQL Host", "127.0.0.1")
		dbPort = w.ask("MySQL Port", "3306")
		dbName = w.ask("Database Name", "padi_db")
		dbUser = w.ask("Database User", "root")
		dbPassword = w.ask("Database Password", "")
	case 3:
		dbDriver = "postgres"
		dbHost = w.ask("PostgreSQL Host", "127.0.0.1")
		dbPort = w.ask("PostgreSQL Port", "5432")
		dbName = w.ask("Database Name", "padi_db")
		dbUser = w.ask("Database User", "postgres")
		dbPassword = w.ask("Database Password", "")
	}

	// 2. Application Config
	appPort := w.ask("Application HTTP Port", "8080")
	appEnv := w.ask("Application Environment (development/production)", "development")

	// 3. Generate Cryptographic JWT Secret
	tokenBytes := make([]byte, 32)
	_, _ = rand.Read(tokenBytes)
	jwtSecret := hex.EncodeToString(tokenBytes)

	// 4. Construct .env content
	var envBuilder strings.Builder
	envBuilder.WriteString("# Application\n")
	envBuilder.WriteString(fmt.Sprintf("APP_NAME=\"PADI REST API GO\"\n"))
	envBuilder.WriteString(fmt.Sprintf("APP_ENV=%s\n", appEnv))
	envBuilder.WriteString(fmt.Sprintf("APP_PORT=%s\n\n", appPort))

	envBuilder.WriteString("# Security & JWT\n")
	envBuilder.WriteString(fmt.Sprintf("JWT_SECRET=%s\n", jwtSecret))
	envBuilder.WriteString("JWT_EXPIRY_HOURS=24\n\n")

	envBuilder.WriteString("# Database Configuration\n")
	envBuilder.WriteString(fmt.Sprintf("DB_CONNECTION=%s\n", dbDriver))
	if dbDriver == "sqlite" {
		envBuilder.WriteString(fmt.Sprintf("DB_DATABASE=%s\n\n", dbName))
	} else {
		envBuilder.WriteString(fmt.Sprintf("DB_HOST=%s\n", dbHost))
		envBuilder.WriteString(fmt.Sprintf("DB_PORT=%s\n", dbPort))
		envBuilder.WriteString(fmt.Sprintf("DB_DATABASE=%s\n", dbName))
		envBuilder.WriteString(fmt.Sprintf("DB_USERNAME=%s\n", dbUser))
		envBuilder.WriteString(fmt.Sprintf("DB_PASSWORD=%s\n\n", dbPassword))
	}

	envBuilder.WriteString("# CORS Settings\n")
	envBuilder.WriteString("CORS_ALLOWED_ORIGINS=\"*\"\n\n")

	envBuilder.WriteString("# Cache & Queue\n")
	envBuilder.WriteString("CACHE_DRIVER=memory\n")
	envBuilder.WriteString("QUEUE_DRIVER=database\n\n")

	envBuilder.WriteString("# File Storage\n")
	envBuilder.WriteString("STORAGE_PATH=storage/uploads\n")

	envFile := filepath.Join(w.baseDir, ".env")
	if err := os.WriteFile(envFile, []byte(envBuilder.String()), 0644); err != nil {
		fmt.Printf(colorRed+"❌ Failed to write .env file: %v\n"+colorReset, err)
		return err
	}

	fmt.Println()
	fmt.Printf(colorGreen+"✓ Configuration successfully written to %s\n"+colorReset, envFile)

	// 5. Test Database Connection
	fmt.Println(colorCyan + "🔌 Testing database connection..." + colorReset)
	cfg := &config.Config{
		DBConnection: dbDriver,
		DBHost:       dbHost,
		DBPort:       dbPort,
		DBDatabase:   dbName,
		DBUsername:   dbUser,
		DBPassword:   dbPassword,
	}

	db, err := database.Connect(cfg)
	if err != nil {
		fmt.Printf(colorYellow+"⚠️  Database connection failed: %v\n"+colorReset, err)
		fmt.Println("Please verify your database credentials in .env")
		return nil
	}
	fmt.Printf(colorGreen+"✓ Successfully connected to database [%s]!\n"+colorReset, dbDriver)

	// 6. Offer immediate migration
	runMig := w.askYesNo("Do you want to run database migrations now?", true)
	if runMig {
		fmt.Println(colorCyan + "🚀 Running pending migrations..." + colorReset)
		if err := migrator.RunPending(db); err != nil {
			fmt.Printf(colorRed+"❌ Migration failed: %v\n"+colorReset, err)
		} else {
			fmt.Println(colorGreen + "✓ Migrations completed successfully!" + colorReset)
		}
	}

	fmt.Println()
	fmt.Println(colorGreen + colorBold + "✨ PADI REST API GO is ready!" + colorReset)
	fmt.Println("Next steps:")
	fmt.Println(colorCyan + "  1. Start server   : " + colorReset + "go run cmd/padi/main.go serve")
	fmt.Println(colorCyan + "  2. Scaffold CRUD  : " + colorReset + "go run cmd/padi/main.go g <table_name>")
	fmt.Println()

	return nil
}

func (w *Wizard) ask(prompt string, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s%s [%s]: %s", colorCyan, prompt, defaultValue, colorReset)
	} else {
		fmt.Printf("%s%s: %s", colorCyan, prompt, colorReset)
	}

	input, _ := w.reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}
	return input
}

func (w *Wizard) askChoice(prompt string, min, max, defaultVal int) int {
	for {
		res := w.ask(prompt, strconv.Itoa(defaultVal))
		val, err := strconv.Atoi(res)
		if err == nil && val >= min && val <= max {
			return val
		}
		fmt.Printf(colorRed+"Invalid option. Please enter a number between %d and %d.\n"+colorReset, min, max)
	}
}

func (w *Wizard) askYesNo(prompt string, defaultVal bool) bool {
	defStr := "y"
	if !defaultVal {
		defStr = "n"
	}
	for {
		res := strings.ToLower(w.ask(fmt.Sprintf("%s (y/n)", prompt), defStr))
		if res == "y" || res == "yes" {
			return true
		}
		if res == "n" || res == "no" {
			return false
		}
	}
}
