package httpapi

import "github.com/bruhjeshhh/delivery-tracker/internal/models"

// ValidTransitions defines the legal forward moves in the order lifecycle.
// Enforced for agent-driven status updates; admin override bypasses this map
// entirely (see handleAdminOverrideStatus) since admins may need to correct
// mistakes out of band.
var ValidTransitions = map[models.OrderStatus][]models.OrderStatus{
	models.StatusCreated:        {models.StatusAssigned, models.StatusCancelled},
	models.StatusAssigned:       {models.StatusPickedUp, models.StatusCancelled},
	models.StatusPickedUp:       {models.StatusInTransit},
	models.StatusInTransit:      {models.StatusOutForDelivery},
	models.StatusOutForDelivery: {models.StatusDelivered, models.StatusFailed},
	models.StatusFailed:         {models.StatusRescheduled},
	models.StatusRescheduled:    {models.StatusAssigned},
}

func isValidTransition(from, to models.OrderStatus) bool {
	for _, allowed := range ValidTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}
