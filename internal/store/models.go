package store

import "time"

type User struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	AvatarURL string    `json:"avatar_url"`
	Role      string    `json:"role,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Household struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Session struct {
	ID          string
	UserID      int64
	HouseholdID int64
	ExpiresAt   time.Time
}

type APIToken struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"user_id"`
	HouseholdID int64      `json:"household_id"`
	Name        string     `json:"name"`
	Prefix      string     `json:"prefix"`
	Scope       string     `json:"scope"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type APITokenWithSecret struct {
	APIToken
	Token string `json:"token"`
}

type Project struct {
	ID          int64      `json:"id"`
	HouseholdID int64      `json:"household_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	CreatedBy   int64      `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type ProjectFolder struct {
	ID          int64     `json:"id"`
	HouseholdID int64     `json:"household_id"`
	ProjectID   int64     `json:"project_id"`
	Title       string    `json:"title"`
	Status      string    `json:"status"`
	SortOrder   int       `json:"sort_order"`
	CreatedBy   int64     `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Task struct {
	ID                     int64      `json:"id"`
	HouseholdID            int64      `json:"household_id"`
	ProjectID              *int64     `json:"project_id,omitempty"`
	ProjectFolderID        *int64     `json:"project_folder_id,omitempty"`
	RoutineID              *int64     `json:"routine_id,omitempty"`
	AssetID                *int64     `json:"asset_id,omitempty"`
	AssetMaintenanceItemID *int64     `json:"asset_maintenance_item_id,omitempty"`
	AssignedTo             *int64     `json:"assigned_to,omitempty"`
	AssignedName           string     `json:"assigned_name,omitempty"`
	Title                  string     `json:"title"`
	Notes                  string     `json:"notes"`
	Status                 string     `json:"status"`
	Priority               string     `json:"priority"`
	DueAt                  *time.Time `json:"due_at,omitempty"`
	CompletedAt            *time.Time `json:"completed_at,omitempty"`
	CreatedBy              int64      `json:"created_by"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type Event struct {
	ID          int64     `json:"id"`
	HouseholdID int64     `json:"household_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	Location    string    `json:"location"`
	Source      string    `json:"source"`
	ExternalID  string    `json:"external_id"`
	SyncStatus  string    `json:"sync_status"`
	CreatedBy   int64     `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Routine struct {
	ID              int64      `json:"id"`
	HouseholdID     int64      `json:"household_id"`
	AssignedTo      *int64     `json:"assigned_to,omitempty"`
	AssignedName    string     `json:"assigned_name,omitempty"`
	Title           string     `json:"title"`
	Notes           string     `json:"notes"`
	Cadence         string     `json:"cadence"`
	Status          string     `json:"status"`
	NextDueAt       *time.Time `json:"next_due_at,omitempty"`
	LastCompletedAt *time.Time `json:"last_completed_at,omitempty"`
	CreatedBy       int64      `json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type HouseholdList struct {
	ID          int64     `json:"id"`
	HouseholdID int64     `json:"household_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Kind        string    `json:"kind"`
	Status      string    `json:"status"`
	CreatedBy   int64     `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ListItem struct {
	ID           int64      `json:"id"`
	HouseholdID  int64      `json:"household_id"`
	ListID       int64      `json:"list_id"`
	AssignedTo   *int64     `json:"assigned_to,omitempty"`
	AssignedName string     `json:"assigned_name,omitempty"`
	Title        string     `json:"title"`
	Notes        string     `json:"notes"`
	Status       string     `json:"status"`
	DueAt        *time.Time `json:"due_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	CreatedBy    int64      `json:"created_by"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type Contact struct {
	ID          int64     `json:"id"`
	HouseholdID int64     `json:"household_id"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"`
	Email       string    `json:"email"`
	Phone       string    `json:"phone"`
	Notes       string    `json:"notes"`
	Status      string    `json:"status"`
	CreatedBy   int64     `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Asset struct {
	ID                         int64      `json:"id"`
	HouseholdID                int64      `json:"household_id"`
	Name                       string     `json:"name"`
	Kind                       string     `json:"kind"`
	SerialNumber               string     `json:"serial_number"`
	WarrantyExpiresAt          *time.Time `json:"warranty_expires_at,omitempty"`
	PurchaseDate               *time.Time `json:"purchase_date,omitempty"`
	PurchaseCost               *float64   `json:"purchase_cost,omitempty"`
	Vendor                     string     `json:"vendor"`
	Model                      string     `json:"model"`
	MaintenanceCadence         string     `json:"maintenance_cadence"`
	MaintenanceNextDueAt       *time.Time `json:"maintenance_next_due_at,omitempty"`
	MaintenanceLastCompletedAt *time.Time `json:"maintenance_last_completed_at,omitempty"`
	Notes                      string     `json:"notes"`
	Status                     string     `json:"status"`
	CreatedBy                  int64      `json:"created_by"`
	CreatedAt                  time.Time  `json:"created_at"`
	UpdatedAt                  time.Time  `json:"updated_at"`
}

type AssetMaintenanceItem struct {
	ID              int64      `json:"id"`
	HouseholdID     int64      `json:"household_id"`
	AssetID         int64      `json:"asset_id"`
	Title           string     `json:"title"`
	Notes           string     `json:"notes"`
	Cadence         string     `json:"cadence"`
	Status          string     `json:"status"`
	NextDueAt       *time.Time `json:"next_due_at,omitempty"`
	LastCompletedAt *time.Time `json:"last_completed_at,omitempty"`
	CreatedBy       int64      `json:"created_by"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type Document struct {
	ID          int64     `json:"id"`
	HouseholdID int64     `json:"household_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	URL         string    `json:"url"`
	Kind        string    `json:"kind"`
	Status      string    `json:"status"`
	FileName    string    `json:"file_name"`
	FilePath    string    `json:"-"`
	ContentType string    `json:"content_type"`
	FileSize    int64     `json:"file_size"`
	CreatedBy   int64     `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type RelatedItem struct {
	LinkID    int64     `json:"link_id,omitempty"`
	Type      string    `json:"type"`
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Subtitle  string    `json:"subtitle,omitempty"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type RelatedDocument struct {
	LinkID   int64    `json:"link_id"`
	Document Document `json:"document"`
}

type RelatedContact struct {
	LinkID  int64   `json:"link_id"`
	Contact Contact `json:"contact"`
}

type RelatedAsset struct {
	LinkID int64 `json:"link_id"`
	Asset  Asset `json:"asset"`
}

type RoutineNotice struct {
	Routine Routine `json:"routine"`
	Kind    string  `json:"kind"`
	Message string  `json:"message"`
}

type DashboardTile struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type ModuleItem struct {
	ID        int64      `json:"id"`
	Title     string     `json:"title,omitempty"`
	Name      string     `json:"name,omitempty"`
	Kind      string     `json:"kind,omitempty"`
	Cadence   string     `json:"cadence,omitempty"`
	NextDueAt *time.Time `json:"next_due_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type Dashboard struct {
	Household      Household       `json:"household"`
	CurrentUser    User            `json:"current_user"`
	Members        []User          `json:"members"`
	Projects       []Project       `json:"projects"`
	Tasks          []Task          `json:"tasks"`
	Events         []Event         `json:"events"`
	Routines       []Routine       `json:"routines"`
	Notices        []RoutineNotice `json:"notices"`
	TileOrder      []string        `json:"tile_order"`
	AvailableTiles []DashboardTile `json:"available_tiles"`
	BudgetAppURL   string          `json:"budget_app_url"`
}

type CalendarEntry struct {
	Type     string     `json:"type"`
	ID       int64      `json:"id"`
	Title    string     `json:"title"`
	URL      string     `json:"url"`
	Time     *time.Time `json:"time,omitempty"`
	Meta     string     `json:"meta,omitempty"`
	Status   string     `json:"status,omitempty"`
	Priority string     `json:"priority,omitempty"`
}

type CalendarDay struct {
	Date    time.Time       `json:"date"`
	InMonth bool            `json:"in_month"`
	IsToday bool            `json:"is_today"`
	Entries []CalendarEntry `json:"entries"`
}

type CalendarMonth struct {
	Month     time.Time     `json:"month"`
	PrevMonth string        `json:"prev_month"`
	NextMonth string        `json:"next_month"`
	Days      []CalendarDay `json:"days"`
}

type RoutineInput struct {
	AssignedTo *int64     `json:"assigned_to"`
	Title      string     `json:"title"`
	Notes      string     `json:"notes"`
	Cadence    string     `json:"cadence"`
	Status     string     `json:"status"`
	NextDueAt  *time.Time `json:"next_due_at"`
}

type ProjectFolderInput struct {
	Title     string `json:"title"`
	SortOrder int    `json:"sort_order"`
}

type HouseholdListInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
	Status      string `json:"status"`
}

type ListItemInput struct {
	AssignedTo *int64     `json:"assigned_to"`
	Title      string     `json:"title"`
	Notes      string     `json:"notes"`
	Status     string     `json:"status"`
	DueAt      *time.Time `json:"due_at"`
}

type ContactInput struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Email  string `json:"email"`
	Phone  string `json:"phone"`
	Notes  string `json:"notes"`
	Status string `json:"status"`
}

type AssetInput struct {
	Name                 string     `json:"name"`
	Kind                 string     `json:"kind"`
	SerialNumber         string     `json:"serial_number"`
	WarrantyExpiresAt    *time.Time `json:"warranty_expires_at"`
	PurchaseDate         *time.Time `json:"purchase_date"`
	PurchaseCost         *float64   `json:"purchase_cost"`
	Vendor               string     `json:"vendor"`
	Model                string     `json:"model"`
	MaintenanceCadence   string     `json:"maintenance_cadence"`
	MaintenanceNextDueAt *time.Time `json:"maintenance_next_due_at"`
	Notes                string     `json:"notes"`
	Status               string     `json:"status"`
}

type AssetMaintenanceInput struct {
	Title     string     `json:"title"`
	Notes     string     `json:"notes"`
	Cadence   string     `json:"cadence"`
	Status    string     `json:"status"`
	NextDueAt *time.Time `json:"next_due_at"`
}

type DocumentInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Kind        string `json:"kind"`
	Status      string `json:"status"`
	FileName    string `json:"file_name"`
	FilePath    string `json:"-"`
	ContentType string `json:"content_type"`
	FileSize    int64  `json:"file_size"`
}

type RelatedItemInput struct {
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
}

type MemberInput struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type APITokenInput struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
}
