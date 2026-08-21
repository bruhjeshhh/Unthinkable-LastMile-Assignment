package models

import "time"

type UserRole string

const (
	RoleCustomer UserRole = "customer"
	RoleAgent    UserRole = "agent"
	RoleAdmin    UserRole = "admin"
)

type OrderType string

const (
	OrderTypeB2B OrderType = "B2B"
	OrderTypeB2C OrderType = "B2C"
)

type PaymentType string

const (
	PaymentPrepaid PaymentType = "PREPAID"
	PaymentCOD     PaymentType = "COD"
)

type AgentStatus string

const (
	AgentAvailable AgentStatus = "available"
	AgentBusy      AgentStatus = "busy"
	AgentOffline   AgentStatus = "offline"
)

// OrderStatus values and their legal forward transitions are enforced in
// the orders service (see orders.ValidTransitions), not just documented here.
type OrderStatus string

const (
	StatusCreated        OrderStatus = "CREATED"
	StatusAssigned       OrderStatus = "ASSIGNED"
	StatusPickedUp       OrderStatus = "PICKED_UP"
	StatusInTransit      OrderStatus = "IN_TRANSIT"
	StatusOutForDelivery OrderStatus = "OUT_FOR_DELIVERY"
	StatusDelivered      OrderStatus = "DELIVERED"
	StatusFailed         OrderStatus = "FAILED"
	StatusRescheduled    OrderStatus = "RESCHEDULED"
	StatusCancelled      OrderStatus = "CANCELLED"
)

type User struct {
	ID           int64     `json:"id"`
	Role         UserRole  `json:"role"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Phone        string    `json:"phone,omitempty"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Zone struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type ZoneArea struct {
	ID      int64  `json:"id"`
	ZoneID  int64  `json:"zone_id"`
	Pincode string `json:"pincode"`
	Label   string `json:"label,omitempty"`
}

type RateCard struct {
	ID           int64     `json:"id"`
	OrderType    OrderType `json:"order_type"`
	FromZoneID   int64     `json:"from_zone_id"`
	ToZoneID     int64     `json:"to_zone_id"`
	BaseFee      float64   `json:"base_fee"`
	RatePerKg    float64   `json:"rate_per_kg"`
	CODSurcharge float64   `json:"cod_surcharge"`
}

type Agent struct {
	UserID         int64       `json:"user_id"`
	Name           string      `json:"name,omitempty"`
	ZoneID         *int64      `json:"zone_id"`
	Status         AgentStatus `json:"status"`
	LastAssignedAt *time.Time  `json:"last_assigned_at,omitempty"`
}

type Order struct {
	ID                 int64       `json:"id"`
	CustomerID         int64       `json:"customer_id"`
	CreatedBy          int64       `json:"created_by"`
	PickupAddress      string      `json:"pickup_address"`
	PickupPincode      string      `json:"pickup_pincode"`
	DropAddress        string      `json:"drop_address"`
	DropPincode        string      `json:"drop_pincode"`
	LengthCM           float64     `json:"length_cm"`
	BreadthCM          float64     `json:"breadth_cm"`
	HeightCM           float64     `json:"height_cm"`
	ActualWeightKg     float64     `json:"actual_weight_kg"`
	VolumetricWeightKg float64     `json:"volumetric_weight_kg"`
	BillableWeightKg   float64     `json:"billable_weight_kg"`
	OrderType          OrderType   `json:"order_type"`
	PaymentType        PaymentType `json:"payment_type"`
	FromZoneID         int64       `json:"from_zone_id"`
	ToZoneID           int64       `json:"to_zone_id"`
	RateCardID         int64       `json:"rate_card_id"`
	Charge             float64     `json:"charge"`
	Status             OrderStatus `json:"status"`
	AgentID            *int64      `json:"agent_id"`
	ScheduledDate      *string     `json:"scheduled_date,omitempty"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
}

type OrderStatusHistory struct {
	ID        int64       `json:"id"`
	OrderID   int64       `json:"order_id"`
	Status    OrderStatus `json:"status"`
	ActorID   *int64      `json:"actor_id"`
	ActorRole UserRole    `json:"actor_role"`
	Note      string      `json:"note,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
}

type Reschedule struct {
	ID          int64     `json:"id"`
	OrderID     int64     `json:"order_id"`
	OldDate     *string   `json:"old_date,omitempty"`
	NewDate     string    `json:"new_date"`
	Reason      string    `json:"reason,omitempty"`
	RequestedBy int64     `json:"requested_by"`
	CreatedAt   time.Time `json:"created_at"`
}
