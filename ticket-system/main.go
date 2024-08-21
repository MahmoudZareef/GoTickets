package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

var db *sql.DB

type Ticket struct {
	ID         int    `json:"id"`
	Name       string `json:"name" binding:"required"`
	Desc       string `json:"desc" binding:"required"`
	Allocation int    `json:"allocation" binding:"required,min=1"`
}

type Purchase struct {
	Quantity int    `json:"quantity" binding:"required,min=1"`
	UserID   string `json:"user_id" binding:"required"`
}

func main() {
	dbHost := os.Getenv("DB_HOST")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	connStr := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbUser, dbPassword, dbName)

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Test database connection
	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Successfully connected to the database!")

	// Check if tickets table exists
	var tableName string
	err = db.QueryRow("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'tickets'").Scan(&tableName)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("Tickets table does not exist. Creating tables...")
			err = createTables()
			if err != nil {
				log.Fatalf("Failed to create tables: %v", err)
			}
			log.Println("Tables created successfully!")
		} else {
			log.Fatalf("Error checking for tickets table: %v", err)
		}
	} else {
		log.Println("Tickets table exists!")
	}

	r := gin.Default()

	r.POST("/tickets", createTicket)
	r.GET("/tickets", getAllTickets)
	r.GET("/tickets/:id", getTicket)
	r.POST("/tickets/:id/purchases", purchaseTicket)

	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}

func createTables() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS tickets (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT NOT NULL,
			allocation INTEGER NOT NULL CHECK (allocation >= 0)
		);

		CREATE TABLE IF NOT EXISTS purchases (
			id SERIAL PRIMARY KEY,
			ticket_id INTEGER NOT NULL REFERENCES tickets(id),
			user_id VARCHAR(255) NOT NULL,
			quantity INTEGER NOT NULL CHECK (quantity > 0)
		);
	`)
	return err
}

func getAllTickets(c *gin.Context) {
	var tickets []Ticket
	rows, err := db.Query("SELECT id, name, description, allocation FROM tickets")
	if err != nil {
		log.Printf("Failed to retrieve tickets: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to retrieve tickets: %v", err)})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var t Ticket
		if err := rows.Scan(&t.ID, &t.Name, &t.Desc, &t.Allocation); err != nil {
			log.Printf("Failed to scan ticket: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to scan ticket: %v", err)})
			return
		}
		tickets = append(tickets, t)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error after scanning all rows: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error after scanning all rows: %v", err)})
		return
	}

	c.JSON(http.StatusOK, tickets)
}

func createTicket(c *gin.Context) {
	var newTicket Ticket
	if err := c.ShouldBindJSON(&newTicket); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query := `INSERT INTO tickets (name, description, allocation) VALUES ($1, $2, $3) RETURNING id`
	err := db.QueryRow(query, newTicket.Name, newTicket.Desc, newTicket.Allocation).Scan(&newTicket.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create ticket"})
		return
	}

	c.JSON(http.StatusCreated, newTicket)
}

func getTicket(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	var ticket Ticket
	query := `SELECT id, name, description, allocation FROM tickets WHERE id = $1`
	err = db.QueryRow(query, id).Scan(&ticket.ID, &ticket.Name, &ticket.Desc, &ticket.Allocation)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve ticket"})
		}
		return
	}

	c.JSON(http.StatusOK, ticket)
}

func purchaseTicket(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	var purchase Purchase
	if err := c.ShouldBindJSON(&purchase); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	var currentAllocation int
	query := `SELECT allocation FROM tickets WHERE id = $1 FOR UPDATE`
	err = tx.QueryRow(query, id).Scan(&currentAllocation)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve ticket"})
		}
		return
	}

	if currentAllocation < purchase.Quantity {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Not enough tickets available"})
		return
	}

	newAllocation := currentAllocation - purchase.Quantity
	_, err = tx.Exec(`UPDATE tickets SET allocation = $1 WHERE id = $2`, newAllocation, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update ticket allocation"})
		return
	}

	_, err = tx.Exec(`INSERT INTO purchases (ticket_id, user_id, quantity) VALUES ($1, $2, $3)`, id, purchase.UserID, purchase.Quantity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record purchase"})
		return
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.Status(http.StatusOK)
}
