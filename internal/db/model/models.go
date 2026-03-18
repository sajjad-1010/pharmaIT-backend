package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type UserRole string

const (
	UserRolePharmacy     UserRole = "PHARMACY"
	UserRoleWholesaler   UserRole = "WHOLESALER"
	UserRoleManufacturer UserRole = "MANUFACTURER"
	UserRoleAdmin        UserRole = "ADMIN"
)

type UserStatus string

const (
	UserStatusPending   UserStatus = "PENDING"
	UserStatusActive    UserStatus = "ACTIVE"
	UserStatusSuspended UserStatus = "SUSPENDED"
)

type OrderStatus string

const (
	OrderStatusCreated   OrderStatus = "CREATED"
	OrderStatusConfirmed OrderStatus = "CONFIRMED"
	OrderStatusPacking   OrderStatus = "PACKING"
	OrderStatusShipped   OrderStatus = "SHIPPED"
	OrderStatusDelivered OrderStatus = "DELIVERED"
	OrderStatusCanceled  OrderStatus = "CANCELED"
)

type RareRequestStatus string

const (
	RareRequestStatusOpen     RareRequestStatus = "OPEN"
	RareRequestStatusInReview RareRequestStatus = "IN_REVIEW"
	RareRequestStatusSelected RareRequestStatus = "SELECTED"
	RareRequestStatusClosed   RareRequestStatus = "CLOSED"
	RareRequestStatusCanceled RareRequestStatus = "CANCELED"
)

type RareBidStatus string

const (
	RareBidStatusSubmitted RareBidStatus = "SUBMITTED"
	RareBidStatusAccepted  RareBidStatus = "ACCEPTED"
	RareBidStatusRejected  RareBidStatus = "REJECTED"
	RareBidStatusWithdrawn RareBidStatus = "WITHDRAWN"
)

type ManufacturerRequestStatus string

const (
	ManufacturerRequestStatusCreated  ManufacturerRequestStatus = "CREATED"
	ManufacturerRequestStatusSent     ManufacturerRequestStatus = "SENT"
	ManufacturerRequestStatusQuoted   ManufacturerRequestStatus = "QUOTED"
	ManufacturerRequestStatusApproved ManufacturerRequestStatus = "APPROVED"
	ManufacturerRequestStatusRejected ManufacturerRequestStatus = "REJECTED"
	ManufacturerRequestStatusClosed   ManufacturerRequestStatus = "CLOSED"
)

type DiscountCampaignStatus string

const (
	DiscountCampaignStatusDraft  DiscountCampaignStatus = "DRAFT"
	DiscountCampaignStatusActive DiscountCampaignStatus = "ACTIVE"
	DiscountCampaignStatusPaused DiscountCampaignStatus = "PAUSED"
	DiscountCampaignStatusEnded  DiscountCampaignStatus = "ENDED"
)

type DiscountType string

const (
	DiscountTypePercent DiscountType = "PERCENT"
	DiscountTypeFixed   DiscountType = "FIXED"
)

type PaymentStatus string

const (
	PaymentStatusPending  PaymentStatus = "PENDING"
	PaymentStatusPaid     PaymentStatus = "PAID"
	PaymentStatusFailed   PaymentStatus = "FAILED"
	PaymentStatusReversed PaymentStatus = "REVERSED"
)

type OutboxStatus string

const (
	OutboxStatusNew       OutboxStatus = "NEW"
	OutboxStatusProcessed OutboxStatus = "PROCESSED"
	OutboxStatusFailed    OutboxStatus = "FAILED"
)

type InventoryMovementType string

const (
	InventoryMovementTypeIn       InventoryMovementType = "IN"
	InventoryMovementTypeOut      InventoryMovementType = "OUT"
	InventoryMovementTypeReserved InventoryMovementType = "RESERVED"
	InventoryMovementTypeReleased InventoryMovementType = "RELEASED"
	InventoryMovementTypeAdjust   InventoryMovementType = "ADJUST"
)

type MedicineCandidateStatus string

const (
	MedicineCandidateStatusPending  MedicineCandidateStatus = "PENDING"
	MedicineCandidateStatusApproved MedicineCandidateStatus = "APPROVED"
	MedicineCandidateStatusRejected MedicineCandidateStatus = "REJECTED"
)

type User struct {
	ID           uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Email        *string    `gorm:"type:text;uniqueIndex"`
	Phone        *string    `gorm:"type:text;uniqueIndex"`
	PasswordHash string     `gorm:"type:text;not null"`
	Role         UserRole   `gorm:"type:user_role;not null"`
	Status       UserStatus `gorm:"type:user_status;not null;default:PENDING"`
	CreatedAt    time.Time  `gorm:"type:timestamptz;not null;default:now()"`
	UpdatedAt    time.Time  `gorm:"type:timestamptz;not null;default:now()"`
}

func (User) TableName() string { return "users" }

type Pharmacy struct {
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name      string    `gorm:"type:text;not null"`
	City      *string   `gorm:"type:text"`
	Address   *string   `gorm:"type:text"`
	LicenseNo *string   `gorm:"type:text"`
}

func (Pharmacy) TableName() string { return "pharmacies" }

type Wholesaler struct {
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name      string    `gorm:"type:text;not null"`
	Country   *string   `gorm:"type:text"`
	City      *string   `gorm:"type:text"`
	Address   *string   `gorm:"type:text"`
	LicenseNo *string   `gorm:"type:text"`
}

func (Wholesaler) TableName() string { return "wholesalers" }

type Manufacturer struct {
	UserID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name           string    `gorm:"type:text;not null"`
	Country        *string   `gorm:"type:text"`
	RegistrationNo *string   `gorm:"type:text"`
}

func (Manufacturer) TableName() string { return "manufacturers" }

type Medicine struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	GenericName string    `gorm:"type:text;not null;index"`
	BrandName   *string   `gorm:"type:text;index"`
	Form        string    `gorm:"type:text;not null"`
	Strength    *string   `gorm:"type:text"`
	PackSize    *string   `gorm:"type:text"`
	ATCCode     *string   `gorm:"type:text"`
	IsActive    bool      `gorm:"type:boolean;not null;default:true"`
	CreatedAt   time.Time `gorm:"type:timestamptz;not null;default:now()"`
	UpdatedAt   time.Time `gorm:"type:timestamptz;not null;default:now()"`
}

func (Medicine) TableName() string { return "medicines" }

type MedicineCandidate struct {
	ID                    uuid.UUID               `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	WholesalerID          uuid.UUID               `gorm:"type:uuid;not null;index:idx_medicine_candidates_wh_status_created,priority:1"`
	GenericName           string                  `gorm:"type:text;not null"`
	BrandName             *string                 `gorm:"type:text"`
	Form                  string                  `gorm:"type:text;not null"`
	Strength              *string                 `gorm:"type:text"`
	PackSize              *string                 `gorm:"type:text"`
	ATCCode               *string                 `gorm:"type:text"`
	NormalizedGenericName string                  `gorm:"type:text;not null"`
	NormalizedBrandName   *string                 `gorm:"type:text"`
	NormalizedForm        string                  `gorm:"type:text;not null"`
	NormalizedStrength    *string                 `gorm:"type:text"`
	Status                MedicineCandidateStatus `gorm:"type:medicine_candidate_status;not null;default:PENDING;index:idx_medicine_candidates_status_created,priority:1;index:idx_medicine_candidates_wh_status_created,priority:2"`
	MatchedMedicineID     *uuid.UUID              `gorm:"type:uuid"`
	AdminDecisionNote     *string                 `gorm:"type:text"`
	ReviewedBy            *uuid.UUID              `gorm:"type:uuid"`
	ReviewedAt            *time.Time              `gorm:"type:timestamptz"`
	CreatedAt             time.Time               `gorm:"type:timestamptz;not null;default:now();index:idx_medicine_candidates_status_created,priority:2;sort:desc;index:idx_medicine_candidates_wh_status_created,priority:3;sort:desc"`
	UpdatedAt             time.Time               `gorm:"type:timestamptz;not null;default:now()"`
}

func (MedicineCandidate) TableName() string { return "medicine_candidates" }

type WholesalerOffer struct {
	ID           uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	WholesalerID uuid.UUID       `gorm:"type:uuid;not null;index:uq_wholesaler_medicine_active_offer,priority:1,unique"`
	MedicineID   uuid.UUID       `gorm:"type:uuid;not null;index:uq_wholesaler_medicine_active_offer,priority:2,unique;index:idx_offers_med_active_updated,priority:1"`
	DisplayPrice decimal.Decimal `gorm:"type:numeric(18,4);not null"`
	Currency     string          `gorm:"type:text;not null"`
	AvailableQty int             `gorm:"type:int;not null;default:0"`
	MinOrderQty  int             `gorm:"type:int;not null;default:1" json:"-"`
	ExpiryDate   *time.Time      `gorm:"type:date"`
	IsActive     bool            `gorm:"type:boolean;not null;default:true;index:uq_wholesaler_medicine_active_offer,priority:3,unique;index:idx_offers_med_active_updated,priority:2;index:idx_offers_wh_active,priority:2"`
	CreatedAt    time.Time       `gorm:"type:timestamptz;not null;default:now()"`
	UpdatedAt    time.Time       `gorm:"type:timestamptz;not null;default:now();index:idx_offers_med_active_updated,priority:3;sort:desc"`
}

func (WholesalerOffer) TableName() string { return "wholesaler_offers" }

type Order struct {
	ID           uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	PharmacyID   uuid.UUID       `gorm:"type:uuid;not null;index:idx_orders_pharmacy_created,priority:1"`
	WholesalerID uuid.UUID       `gorm:"type:uuid;not null;index:idx_orders_wholesaler_created,priority:1"`
	Status       OrderStatus     `gorm:"type:order_status;not null;default:CREATED;index"`
	TotalAmount  decimal.Decimal `gorm:"type:numeric(18,4);not null;default:0"`
	Currency     string          `gorm:"type:text;not null"`
	CreatedAt    time.Time       `gorm:"type:timestamptz;not null;default:now();index:idx_orders_pharmacy_created,priority:2;sort:desc;index:idx_orders_wholesaler_created,priority:2;sort:desc"`
	UpdatedAt    time.Time       `gorm:"type:timestamptz;not null;default:now()"`
}

func (Order) TableName() string { return "orders" }

type OrderItem struct {
	ID         uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	OrderID    uuid.UUID       `gorm:"type:uuid;not null;index"`
	MedicineID uuid.UUID       `gorm:"type:uuid;not null;index"`
	Qty        int             `gorm:"type:int;not null"`
	UnitPrice  decimal.Decimal `gorm:"type:numeric(18,4);not null"`
	LineTotal  decimal.Decimal `gorm:"type:numeric(18,4);not null"`
}

func (OrderItem) TableName() string { return "order_items" }

type RareRequest struct {
	ID                uuid.UUID         `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	PharmacyID        uuid.UUID         `gorm:"type:uuid;not null"`
	MedicineID        *uuid.UUID        `gorm:"type:uuid"`
	RequestedNameText *string           `gorm:"type:text"`
	Qty               int               `gorm:"type:int;not null"`
	DeadlineAt        time.Time         `gorm:"type:timestamptz;not null;index:idx_rare_requests_status_deadline,priority:2"`
	Notes             *string           `gorm:"type:text"`
	Status            RareRequestStatus `gorm:"type:rare_request_status;not null;default:OPEN;index:idx_rare_requests_status_deadline,priority:1"`
	CreatedAt         time.Time         `gorm:"type:timestamptz;not null;default:now()"`
	UpdatedAt         time.Time         `gorm:"type:timestamptz;not null;default:now()"`
}

func (RareRequest) TableName() string { return "rare_requests" }

type RareBid struct {
	ID               uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	RareRequestID    uuid.UUID       `gorm:"type:uuid;not null;index:idx_rare_bids_request_status,priority:1"`
	WholesalerID     uuid.UUID       `gorm:"type:uuid;not null"`
	Price            decimal.Decimal `gorm:"type:numeric(18,4);not null"`
	Currency         string          `gorm:"type:text;not null"`
	AvailableQty     int             `gorm:"type:int;not null;default:0"`
	DeliveryETAHours *int            `gorm:"type:int"`
	Notes            *string         `gorm:"type:text"`
	Status           RareBidStatus   `gorm:"type:rare_bid_status;not null;default:SUBMITTED;index:idx_rare_bids_request_status,priority:2"`
	CreatedAt        time.Time       `gorm:"type:timestamptz;not null;default:now()"`
	UpdatedAt        time.Time       `gorm:"type:timestamptz;not null;default:now()"`
}

func (RareBid) TableName() string { return "rare_bids" }

type ManufacturerRequest struct {
	ID                uuid.UUID                 `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	WholesalerID      uuid.UUID                 `gorm:"type:uuid;not null;index:idx_mr_wholesaler_status,priority:1"`
	ManufacturerID    uuid.UUID                 `gorm:"type:uuid;not null;index:idx_mr_manufacturer_status_created,priority:1"`
	MedicineID        *uuid.UUID                `gorm:"type:uuid"`
	RequestedNameText *string                   `gorm:"type:text"`
	Qty               int                       `gorm:"type:int;not null"`
	NeededBy          *time.Time                `gorm:"type:timestamptz"`
	Status            ManufacturerRequestStatus `gorm:"type:manufacturer_request_status;not null;default:CREATED;index:idx_mr_manufacturer_status_created,priority:2;index:idx_mr_wholesaler_status,priority:2"`
	CreatedAt         time.Time                 `gorm:"type:timestamptz;not null;default:now();index:idx_mr_manufacturer_status_created,priority:3;sort:desc"`
	UpdatedAt         time.Time                 `gorm:"type:timestamptz;not null;default:now()"`
}

func (ManufacturerRequest) TableName() string { return "manufacturer_requests" }

type ManufacturerQuote struct {
	ID             uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	RequestID      uuid.UUID       `gorm:"type:uuid;not null;index"`
	ManufacturerID uuid.UUID       `gorm:"type:uuid;not null"`
	UnitPriceFinal decimal.Decimal `gorm:"type:numeric(18,4);not null"`
	Currency       string          `gorm:"type:text;not null"`
	LeadTimeDays   *int            `gorm:"type:int"`
	ValidUntil     *time.Time      `gorm:"type:timestamptz"`
	Notes          *string         `gorm:"type:text"`
	CreatedAt      time.Time       `gorm:"type:timestamptz;not null;default:now()"`
}

func (ManufacturerQuote) TableName() string { return "manufacturer_quotes" }

type DiscountCampaign struct {
	ID           uuid.UUID              `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	WholesalerID uuid.UUID              `gorm:"type:uuid;not null"`
	Title        string                 `gorm:"type:text;not null"`
	StartsAt     *time.Time             `gorm:"type:timestamptz"`
	EndsAt       *time.Time             `gorm:"type:timestamptz"`
	Status       DiscountCampaignStatus `gorm:"type:discount_campaign_status;not null;default:DRAFT"`
}

func (DiscountCampaign) TableName() string { return "discount_campaigns" }

type DiscountItem struct {
	ID            uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	CampaignID    uuid.UUID       `gorm:"type:uuid;not null"`
	MedicineID    uuid.UUID       `gorm:"type:uuid;not null"`
	DiscountType  DiscountType    `gorm:"type:discount_type;not null"`
	DiscountValue decimal.Decimal `gorm:"type:numeric(18,4);not null"`
}

func (DiscountItem) TableName() string { return "discount_items" }

type Payment struct {
	ID            uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID        uuid.UUID       `gorm:"type:uuid;not null;index:idx_payments_user_created,priority:1"`
	Amount        decimal.Decimal `gorm:"type:numeric(18,4);not null"`
	Currency      string          `gorm:"type:text;not null"`
	InvoiceID     string          `gorm:"type:text;not null;uniqueIndex"`
	TransactionID *string         `gorm:"type:text;uniqueIndex"`
	Status        PaymentStatus   `gorm:"type:payment_status;not null;default:PENDING"`
	PaidAt        *time.Time      `gorm:"type:timestamptz"`
	CreatedAt     time.Time       `gorm:"type:timestamptz;not null;default:now();index:idx_payments_user_created,priority:2;sort:desc"`
}

func (Payment) TableName() string { return "payments" }

type AccessPass struct {
	UserID      uuid.UUID `gorm:"type:uuid;primaryKey"`
	AccessUntil time.Time `gorm:"type:timestamptz;not null"`
}

func (AccessPass) TableName() string { return "access_passes" }

type Outbox struct {
	ID          uuid.UUID    `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	EventType   string       `gorm:"type:text;not null"`
	PayloadJSON []byte       `gorm:"type:jsonb;not null"`
	Status      OutboxStatus `gorm:"type:outbox_status;not null;default:NEW;index:idx_outbox_status_created,priority:1"`
	CreatedAt   time.Time    `gorm:"type:timestamptz;not null;default:now();index:idx_outbox_status_created,priority:2"`
	ProcessedAt *time.Time   `gorm:"type:timestamptz"`
}

func (Outbox) TableName() string { return "outbox" }

type InventoryMovement struct {
	ID           uuid.UUID             `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	WholesalerID uuid.UUID             `gorm:"type:uuid;not null;index:idx_inventory_wh_medicine_created,priority:1"`
	MedicineID   uuid.UUID             `gorm:"type:uuid;not null;index:idx_inventory_wh_medicine_created,priority:2"`
	Type         InventoryMovementType `gorm:"type:inventory_movement_type;not null"`
	Qty          int                   `gorm:"type:int;not null"`
	RefType      *string               `gorm:"type:text"`
	RefID        *uuid.UUID            `gorm:"type:uuid"`
	CreatedAt    time.Time             `gorm:"type:timestamptz;not null;default:now();index:idx_inventory_wh_medicine_created,priority:3;sort:desc"`
}

func (InventoryMovement) TableName() string { return "inventory_movements" }
