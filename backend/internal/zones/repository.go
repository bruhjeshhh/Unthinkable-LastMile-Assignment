package zones

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bruhjeshhh/delivery-tracker/internal/models"
)

var ErrPincodeNotMapped = errors.New("pincode is not mapped to any zone; ask admin to configure it")

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// DetectZone is the "zone detection" step: a direct pincode -> zone lookup
// against the admin-configured zone_areas table. O(1) via unique index on
// pincode. If admin hasn't mapped this pincode yet, order creation fails
// with a clear, actionable error rather than guessing.
func (r *Repository) DetectZone(ctx context.Context, pincode string) (int64, error) {
	var zoneID int64
	err := r.db.QueryRow(ctx,
		`SELECT zone_id FROM zone_areas WHERE pincode = $1`, pincode,
	).Scan(&zoneID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrPincodeNotMapped
	}
	if err != nil {
		return 0, err
	}
	return zoneID, nil
}

func (r *Repository) ListZones(ctx context.Context) ([]models.Zone, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name FROM zones ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Zone
	for rows.Next() {
		var z models.Zone
		if err := rows.Scan(&z.ID, &z.Name); err != nil {
			return nil, err
		}
		out = append(out, z)
	}
	return out, rows.Err()
}

func (r *Repository) CreateZone(ctx context.Context, name string) (models.Zone, error) {
	var z models.Zone
	err := r.db.QueryRow(ctx,
		`INSERT INTO zones (name) VALUES ($1) RETURNING id, name`, name,
	).Scan(&z.ID, &z.Name)
	return z, err
}

func (r *Repository) MapPincode(ctx context.Context, zoneID int64, pincode, label string) (models.ZoneArea, error) {
	var za models.ZoneArea
	err := r.db.QueryRow(ctx,
		`INSERT INTO zone_areas (zone_id, pincode, label) VALUES ($1, $2, $3)
		 ON CONFLICT (pincode) DO UPDATE SET zone_id = EXCLUDED.zone_id, label = EXCLUDED.label
		 RETURNING id, zone_id, pincode, COALESCE(label, '')`,
		zoneID, pincode, label,
	).Scan(&za.ID, &za.ZoneID, &za.Pincode, &za.Label)
	return za, err
}

func (r *Repository) GetRateCard(ctx context.Context, orderType models.OrderType, fromZone, toZone int64) (models.RateCard, error) {
	var c models.RateCard
	err := r.db.QueryRow(ctx,
		`SELECT id, order_type, from_zone_id, to_zone_id, base_fee, rate_per_kg, cod_surcharge
		 FROM rate_cards WHERE order_type = $1 AND from_zone_id = $2 AND to_zone_id = $3`,
		orderType, fromZone, toZone,
	).Scan(&c.ID, &c.OrderType, &c.FromZoneID, &c.ToZoneID, &c.BaseFee, &c.RatePerKg, &c.CODSurcharge)
	return c, err
}

func (r *Repository) UpsertRateCard(ctx context.Context, c models.RateCard) (models.RateCard, error) {
	err := r.db.QueryRow(ctx,
		`INSERT INTO rate_cards (order_type, from_zone_id, to_zone_id, base_fee, rate_per_kg, cod_surcharge)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (order_type, from_zone_id, to_zone_id)
		 DO UPDATE SET base_fee = EXCLUDED.base_fee, rate_per_kg = EXCLUDED.rate_per_kg, cod_surcharge = EXCLUDED.cod_surcharge
		 RETURNING id`,
		c.OrderType, c.FromZoneID, c.ToZoneID, c.BaseFee, c.RatePerKg, c.CODSurcharge,
	).Scan(&c.ID)
	return c, err
}
