# Ticket System

A simple ticket allocation and purchasing system built with Go and PostgreSQL.

## Prerequisites

- Go 1.19 or later
- Docker and Docker Compose
- PostgreSQL

## Setup

1. Clone the repository:
   ```
   git clone https://github.com/yourusername/ticket-system.git
   cd ticket-system
   ```

2. Set up environment variables:
   Create a `.env` file in the project root and add:
   ```
   DB_HOST=db
   DB_USER=postgres
   DB_PASSWORD=postgres
   DB_NAME=ticket_db
   ```

3. Build and run with Docker Compose:
   ```
   docker-compose up --build
   ```

The API will be available at `http://localhost:8082`.

## API Endpoints

- `POST /tickets`: Create a new ticket
- `GET /tickets`: Get all tickets
- `GET /tickets/:id`: Get a specific ticket
- `POST /tickets/:id/purchases`: Purchase tickets

## Running Tests

(To be implemented)

## OpenAPI Specification

(To be implemented)

## License

[MIT License](LICENSE)
