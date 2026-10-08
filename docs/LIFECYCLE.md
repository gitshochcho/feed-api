# Code trace: what runs when someone hits `POST /vendors`

This guide follows the code **in the exact order it runs**. Each step shows the file, the line numbers, the real code, what it does, and where it goes next.

There are two parts:
- **Part A: server start.** Runs **once** when you start the app. It prepares everything.
- **Part B: a user hits the URL.** Runs **every time** a request comes in.

Example request used in this guide:
```
POST http://localhost:8080/vendors
Content-Type: application/json

{"name":"Acme Corp","external_id":"acme-001"}
```

---

## PART A: Server start (`go run ./cmd`)

### A1. Program starts in `main()`
**File:** [cmd/main.go](../cmd/main.go#L13-L15), lines 13–15
```go
func main() {
	// 1. Load Config & Connect DB
	cfg := config.LoadConfig()
```
**What it does:** Go always starts from `func main()`. The first thing it does is load the settings.
**Next →** `config.LoadConfig()` in `internal/config/config.go`.

### A2. Read the `.env` file
**File:** [internal/config/config.go](../internal/config/config.go#L22-L26), lines 22–26
```go
func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}
```
**What it does:** It opens `.env` from the folder you ran the command in, and loads each line (`DB_HOST=localhost`, ...) into environment variables.
**Checkpoint:** if `.env` is missing, the app does not stop. It only prints the warning and uses the defaults below.

### A3. Put the settings into a struct
**File:** [internal/config/config.go](../internal/config/config.go#L28-L38), lines 28–38
```go
	return &Config{
		DBHost:      getEnv("DB_HOST", "localhost"),
		DBPort:      getEnv("DB_PORT", "5432"),
		DBUser:      getEnv("DB_USER", "postgres"),
		DBPassword:  getEnv("DB_PASSWORD", ""),
		DBName:      getEnv("DB_NAME", "postgres"),
		DBSSLMode:   getEnv("DB_SSLMODE", "disable"),
		AutoMigrate: getEnv("DB_AUTO_MIGRATE", "false") == "true",
		ServerPort:  getEnv("SERVER_PORT", "8080"),
		GinMode:     getEnv("GIN_MODE", "debug"),
	}
```
**What it does:** It reads each variable. If a variable is not set, it uses the second value (the default). The result is `cfg`, which holds all the settings.
**Next →** back in `cmd/main.go`.

### A4. Set the Gin mode
**File:** [cmd/main.go](../cmd/main.go#L17), line 17
```go
	gin.SetMode(cfg.GinMode)
```
**What it does:** `debug` prints extra route info, and `release` is quiet (use it on the live server).

### A5. Connect to PostgreSQL
**File:** [cmd/main.go](../cmd/main.go#L19), line 19
```go
	db := database.ConnectDB(cfg)
```
**Next →** `internal/database/db.go`.

**File:** [internal/database/db.go](../internal/database/db.go#L15-L22), lines 15–22
```go
func ConnectDB(cfg *config.Config) *gorm.DB {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		// Converts driver errors (e.g. unique violations) into gorm.ErrDuplicatedKey etc.
		TranslateError: true,
	})
```
**What it does:**
- It builds the connection string (`dsn`) from the settings.
- It opens GORM with the Postgres driver.
- `TranslateError: true` matters later in **B9**. It turns the Postgres "duplicate" error into `gorm.ErrDuplicatedKey`.

**File:** [internal/database/db.go](../internal/database/db.go#L32-L41), lines 32–41
```go
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("Database ping failed: %v\n", err)
	}

	log.Println("Successfully connected to PostgreSQL!")
	return db
```
**What it does:**
- It creates a **connection pool**: at most 25 open connections, shared by all requests.
- `Ping()` makes the first real connection to the database.

**Checkpoint:** if the password, host or database name is wrong, the app **stops here** with `Database ping failed: ...`.
**Next →** back in `main.go`, holding a working `db`.

### A6. (Optional) Create or update tables
**File:** [cmd/main.go](../cmd/main.go#L28-L30), lines 28–30
```go
	if cfg.AutoMigrate {
		database.Migrate(db)
	}
```
**What it does:** It only runs when `DB_AUTO_MIGRATE=true`. `Migrate` (in `db.go`, lines 45–50) calls `db.AutoMigrate(&model.Vendor{})` to create the `vendors` table from the model.
**Checkpoint:** keep this `false` when another app owns the table.

### A7. Build the router and connect the layers
**File:** [cmd/main.go](../cmd/main.go#L33), line 33
```go
	r := routes.SetupRouter(db)
```
**Next →** `internal/routes/routes.go`.

**File:** [internal/routes/routes.go](../internal/routes/routes.go#L12-L21), lines 12–21
```go
func SetupRouter(db *gorm.DB) *gin.Engine {
	r := gin.Default()

	// Vendors
	vendorRepo := repository.NewVendorRepository(db)
	vendorHandler := handler.NewVendorHandler(vendorRepo)
	RegisterVendorRoutes(r.Group("/vendors"), vendorHandler)

	return r
}
```
**What it does, line by line:**
1. `gin.Default()` creates the web server with two built-in middlewares:
   - **Logger** prints one line per request.
   - **Recovery** catches crashes and returns a 500.
2. `NewVendorRepository(db)` creates the repository and gives it the `db`.
   - **File:** [internal/repository/vendor_repository.go](../internal/repository/vendor_repository.go#L15-L17), lines 15–17: `return &VendorRepository{db: db}`
3. `NewVendorHandler(vendorRepo)` creates the handler and gives it the repository.
   - **File:** [internal/handler/vendor_handler.go](../internal/handler/vendor_handler.go#L20-L22), lines 20–22: `return &VendorHandler{repo: repo}`
4. `RegisterVendorRoutes(r.Group("/vendors"), vendorHandler)` makes a group whose URL starts with `/vendors`, then registers the routes in it.

The chain is now: **db → repository → handler → route**.

### A8. Register the URL
**File:** [internal/routes/vendor_routes.go](../internal/routes/vendor_routes.go#L10-L12), lines 10–12
```go
func RegisterVendorRoutes(rg *gin.RouterGroup, h *handler.VendorHandler) {
	rg.POST("", h.CreateVendor)
}
```
**What it does:** It tells Gin that a **POST** to `/vendors` + `""` = `/vendors` must call the function `h.CreateVendor`.
Nothing runs yet. Gin only saves this rule in its route table.

### A9. Start listening
**File:** [cmd/main.go](../cmd/main.go#L36-L39), lines 36–39
```go
	log.Printf("Server starting on port %s...", cfg.ServerPort)
	if err := r.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
```
**What it does:** It opens port 8080 and **waits forever** for requests. `main()` stays on this line while the server is running.
**Checkpoint:** if the port is already used by another program, it stops with `Failed to start server`.

The server is now ready. Part A is finished.

---

## PART B: The user hits `POST /vendors`

### B1. The request reaches Gin
The request arrives on port 8080. On the live server it first goes through Nginx (`https://api.example.com` → `127.0.0.1:8080`).
Gin runs a new goroutine for this request, so many requests can run at the same time.

### B2. Middleware runs first
These come from `gin.Default()` in `internal/routes/routes.go`, line 13. There is no project code here.
- **Logger** starts a timer. At the end it prints: `[GIN] 201 | 7ms | POST "/vendors"`
- **Recovery** watches for a panic and returns a 500 if one happens.

### B3. Gin finds the matching route
Gin looks in its route table, which was filled in step **A8**:
```
POST /vendors  →  h.CreateVendor
```
**Checkpoints:**
- `GET /vendors` → **404** (no GET route is registered)
- `POST /vendors/` (with a slash at the end) → **307** redirect to `/vendors`
- `POST /vendor` (spelled wrong) → **404**

**Next →** `CreateVendor` in the handler.

### B4. The handler starts
**File:** [internal/handler/vendor_handler.go](../internal/handler/vendor_handler.go#L24-L25), lines 24–25
```go
func (h *VendorHandler) CreateVendor(c *gin.Context) {
	var req dto.CreateVendorRequest
```
**What it does:**
- `c` (the `gin.Context`) holds everything about this request: the body, headers and URL. It is also how we send the response.
- `req` is an empty struct that will hold the JSON. Its shape comes from the DTO:

**File:** [internal/dto/vendor_dto.go](../internal/dto/vendor_dto.go#L3-L6), lines 3–6
```go
type CreateVendorRequest struct {
	Name       string `json:"name" binding:"required"`
	ExternalID string `json:"external_id" binding:"required"`
}
```
- `json:"name"` means the JSON key `name` goes into `Name`.
- `binding:"required"` means the field must be present and not empty.

### B5. Read and validate the JSON
**File:** [internal/handler/vendor_handler.go](../internal/handler/vendor_handler.go#L27-L31), lines 27–31
```go
	// 1. Validate JSON input
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request. 'name' and 'external_id' are required."})
		return
	}
```
**What it does:**
1. It reads the request body.
2. It parses the JSON into `req`, so `req.Name = "Acme Corp"` and `req.ExternalID = "acme-001"`.
3. It checks the `binding:"required"` rules.

**Checkpoint, which returns 400 and stops here:**
- broken JSON
- a missing or empty `name` or `external_id`
- a wrong type, for example `"name": 123`

`return` ends the function, so the database is never touched.

### B6. Build the database model
**File:** [internal/handler/vendor_handler.go](../internal/handler/vendor_handler.go#L33-L36), lines 33–36
```go
	vendor := model.Vendor{
		Name:       req.Name,
		ExternalID: req.ExternalID,
	}
```
**What it does:** It copies the data from the request struct into the database struct. `ID` is `0` and `CreatedAt` is empty for now.

The model maps to the `vendors` table:

**File:** [internal/model/vendor.go](../internal/model/vendor.go#L5-L10), lines 5–10
```go
type Vendor struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"size:255;not null;uniqueIndex" json:"name"`
	ExternalID string    `gorm:"size:255;not null;uniqueIndex" json:"external_id"`
	CreatedAt  time.Time `gorm:"not null" json:"created_at"`
}
```
- Struct `Vendor` → table `vendors`. GORM makes the name plural and lowercase.
- Field `ExternalID` → column `external_id`.
- `gorm:"..."` tags describe the column, and `json:"..."` tags set the names used in the response.

### B7. Call the repository
**File:** [internal/handler/vendor_handler.go](../internal/handler/vendor_handler.go#L38-L39), lines 38–39
```go
	// 2. Insert into database (GORM sets ID and CreatedAt)
	if err := h.repo.Create(c.Request.Context(), &vendor); err != nil {
```
**What it does:**
- `h.repo` is the repository that was given to the handler in **A7**.
- `c.Request.Context()` is passed along so the query is cancelled if the user disconnects.
- `&vendor` passes a pointer, so the repository can **fill in** `ID` and `CreatedAt` on this same variable.

**Next →** the repository.

### B8. GORM inserts into PostgreSQL
**File:** [internal/repository/vendor_repository.go](../internal/repository/vendor_repository.go#L20-L22), lines 20–22
```go
func (r *VendorRepository) Create(ctx context.Context, vendor *model.Vendor) error {
	return r.db.WithContext(ctx).Create(vendor).Error
}
```
**What GORM does inside `.Create(vendor)`, in order:**
1. It sets `vendor.CreatedAt = time.Now()`, because the field is named `CreatedAt`.
2. It leaves `ID` out, because it is 0 and the database will create it.
3. It takes a connection from the pool created in **A5**.
4. It runs this SQL, which is the real SQL from the log:
   ```sql
   BEGIN;
   INSERT INTO "vendors" ("name","external_id","created_at")
   VALUES ($1,$2,$3) RETURNING "id";
   COMMIT;
   ```
   The values go in separately as `$1`, `$2`, `$3`, which makes SQL injection impossible.
5. **PostgreSQL checks the rules:**
   - `name` must not be NULL, and must be **unique**
   - `external_id` must not be NULL, and must be **unique**
6. If OK, Postgres saves the row, creates a new `id` (for example `1019`) and sends it back. GORM puts it into `vendor.ID`.
7. If not OK, nothing is saved (ROLLBACK) and an error is returned.

**Next →** back in the handler with `err` (nil = success).

### B9. Handle errors
**File:** [internal/handler/vendor_handler.go](../internal/handler/vendor_handler.go#L39-L47), lines 39–47
```go
	if err := h.repo.Create(c.Request.Context(), &vendor); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			c.JSON(http.StatusConflict, gin.H{"error": "A vendor with this name or external_id already exists."})
			return
		}
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert vendor into database"})
		return
	}
```
**What it does:**
- **Duplicate** `name` or `external_id`: Postgres error `23505` becomes `gorm.ErrDuplicatedKey` (thanks to `TranslateError` in A5), and the handler returns **409 Conflict**.
- **Any other error** (DB down, timeout): the real error goes to the log only, and the user gets a general **500** message.

In both cases `return` stops the function.

### B10. Send the success response
**File:** [internal/handler/vendor_handler.go](../internal/handler/vendor_handler.go#L49-L53), lines 49–53
```go
	// 3. Return success
	c.JSON(http.StatusCreated, gin.H{
		"message": "Vendor created successfully",
		"data":    vendor,
	})
```
**What it does:** It sends **201 Created**. `vendor` now has the `id` and `created_at` that GORM filled in during B8. They are converted to JSON using the `json:"..."` tags:
```json
{
  "message": "Vendor created successfully",
  "data": { "id": 1019, "name": "Acme Corp", "external_id": "acme-001", "created_at": "2026-10-07T16:32:51+06:00" }
}
```

### B11. The request ends
- The Logger from B2 prints: `[GIN] 2026/10/07 - 16:32:51 | 201 | 7.73ms | ::1 | POST "/vendors"`
- The DB connection goes back to the pool for the next request.
- `main()` is still waiting at **A9** for the next request.

---

## Quick map: one request, all files in order

| Step | File | Lines | What happens |
|------|------|-------|--------------|
| B2 | `internal/routes/routes.go` | 13 | Gin middleware (Logger, Recovery) |
| B3 | `internal/routes/vendor_routes.go` | 11 | `POST /vendors` → `CreateVendor` |
| B4 | `internal/handler/vendor_handler.go` + `internal/dto/vendor_dto.go` | 24–25, 3–6 | Prepare the request struct |
| B5 | `internal/handler/vendor_handler.go` | 27–31 | Parse and validate JSON → **400** |
| B6 | `internal/handler/vendor_handler.go` + `internal/model/vendor.go` | 33–36, 5–10 | Build the DB model |
| B7 | `internal/handler/vendor_handler.go` | 38–39 | Call the repository |
| B8 | `internal/repository/vendor_repository.go` | 20–22 | GORM `INSERT` into PostgreSQL |
| B9 | `internal/handler/vendor_handler.go` | 39–47 | Duplicate → **409**, other → **500** |
| B10 | `internal/handler/vendor_handler.go` | 49–53 | Success → **201** |

## How to follow it yourself
1. **With the debugger:** in VS Code, open the Run and Debug panel, choose "Go: Launch Package" with the program set to `./cmd`, and put breakpoints at:
   - `vendor_handler.go` line 28 (B5)
   - `vendor_handler.go` line 39 (B7)
   - `vendor_repository.go` line 21 (B8)

   Then send the curl request and press F10/F11 to walk step by step.
2. **See the SQL:** in `vendor_repository.go` line 21, change `r.db.WithContext(ctx)` to `r.db.Debug().WithContext(ctx)` temporarily. Every query is printed with its values.
3. **Check the row:** `SELECT * FROM vendors ORDER BY id DESC LIMIT 5;`

## Go-live, short checklist
The full commands are in README → "Host live".
1. Test locally: `go vet ./...`, `go build ./...`, `go run ./cmd`, then curl for 201, 400 and 409.
2. Build for Linux (PowerShell): `$env:GOOS="linux"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"; go build -o bin/feed-api ./cmd`.
3. Upload: `scp bin/feed-api deploy@SERVER_IP:/tmp/feed-api`.
4. On the server:
   ```sh
   sudo cp /opt/feed-api/feed-api /opt/feed-api/feed-api.prev
   sudo mv /tmp/feed-api /opt/feed-api/feed-api
   sudo systemctl restart feed-api
   ```
5. Check: `systemctl status feed-api`, `journalctl -u feed-api -n 50`, and a curl to the live URL.
6. If broken, roll back: `sudo mv /opt/feed-api/feed-api.prev /opt/feed-api/feed-api && sudo systemctl restart feed-api`.
