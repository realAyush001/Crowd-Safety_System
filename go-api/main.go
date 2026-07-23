package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()
var rdb = redis.NewClient(&redis.Options{Addr: "localhost:6379"})
var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

type ZoneUpdate struct {
	ZoneID          string `json:"zone_id"`
	Name            string `json:"name"`
	CurrentCount    string `json:"current_count"`
	Capacity        string `json:"capacity"`
	RiskLevel       string `json:"risk_level"`
	EtaDanger       string `json:"eta_danger"`
	CascadeWarning  string `json:"cascade_warning,omitempty"`
	NearestSafeZone string `json:"nearest_safe_zone,omitempty"`
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "🚨 Crowd Safety System API running!")
}

// Returns all 4 zones' current data - used to load the dashboard initially
func zonesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	zoneIDs := []string{"A", "B", "C", "D"}
	var zones []map[string]string
	for _, id := range zoneIDs {
		data, err := rdb.HGetAll(ctx, "zone:"+id).Result()
		if err == nil && len(data) > 0 {
			data["zone_id"] = id
			zones = append(zones, data)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(zones)
}

// Returns a zone's history over time - for the post-event analytics feature
func analyticsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	zoneID := r.URL.Query().Get("zone_id")
	if zoneID == "" {
		http.Error(w, "zone_id is required", http.StatusBadRequest)
		return
	}
	key := fmt.Sprintf("zone:%s:history", zoneID)
	results, err := rdb.ZRangeWithScores(ctx, key, 0, -1).Result()
	if err != nil {
		http.Error(w, "no history found", http.StatusNotFound)
		return
	}

	type Point struct {
		Timestamp int64 `json:"timestamp"`
		Count     int   `json:"count"`
	}
	var points []Point
	for _, z := range results {
		member := z.Member.(string)
		parts := strings.Split(member, ":")
		if len(parts) == 2 {
			count, _ := strconv.Atoi(parts[1])
			points = append(points, Point{Timestamp: int64(z.Score), Count: count})
		}
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Timestamp < points[j].Timestamp })

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(points)
}

// Uses Redis GEO search to find the nearest zone that's currently "Safe"
func findNearestSafeZone(excludeZoneID string) string {
	pos, err := rdb.GeoPos(ctx, "zones_locations", "zone:"+excludeZoneID).Result()
	if err != nil || len(pos) == 0 || pos[0] == nil {
		return ""
	}

	results, err := rdb.GeoSearchLocation(ctx, "zones_locations", &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude:  pos[0].Longitude,
			Latitude:   pos[0].Latitude,
			Radius:     5,
			RadiusUnit: "km",
			Sort:       "ASC",
		},
	}).Result()
	if err != nil {
		return ""
	}

	for _, loc := range results {
		if loc.Name == "zone:"+excludeZoneID {
			continue
		}
		risk, _ := rdb.HGet(ctx, loc.Name, "risk_level").Result()
		if risk == "Safe" {
			return loc.Name
		}
	}
	return ""
}

// Checks if a zone's neighbors are also getting risky - the cascade prediction feature
func checkCascadeRisk(zoneID string) string {
	neighborsStr, err := rdb.Get(ctx, "zone:"+zoneID+":neighbors").Result()
	if err != nil || neighborsStr == "" {
		return ""
	}
	var risky []string
	for _, n := range strings.Split(neighborsStr, ",") {
		risk, _ := rdb.HGet(ctx, "zone:"+n, "risk_level").Result()
		if risk == "Caution" || risk == "Danger" {
			risky = append(risky, n)
		}
	}
	if len(risky) > 0 {
		return "Zone(s) " + strings.Join(risky, ", ") + " may get overcrowded next"
	}
	return ""
}

// The live dashboard feed - listens to ALL zone updates and pushes to the browser
func dashboardLiveHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	pubsub := rdb.PSubscribe(ctx, "zone_updates:*")
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		parts := strings.Split(msg.Channel, ":")
		zoneID := parts[len(parts)-1]

		payloadParts := strings.Split(msg.Payload, ",")
		riskLevel := payloadParts[0]
		eta, count := "", ""
		if len(payloadParts) >= 3 {
			eta = payloadParts[1]
			count = payloadParts[2]
		}

		zoneData, _ := rdb.HGetAll(ctx, "zone:"+zoneID).Result()

		update := ZoneUpdate{
			ZoneID:       zoneID,
			Name:         zoneData["name"],
			CurrentCount: count,
			Capacity:     zoneData["capacity"],
			RiskLevel:    riskLevel,
			EtaDanger:    eta,
		}

		if riskLevel == "Danger" || riskLevel == "Caution" {
			update.CascadeWarning = checkCascadeRisk(zoneID)
		}
		if riskLevel == "Danger" {
			update.NearestSafeZone = findNearestSafeZone(zoneID)
		}

		msgBytes, _ := json.Marshal(update)
		if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
			break
		}
	}
}

func main() {
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/zones", zonesHandler)
	http.HandleFunc("/zone/analytics", analyticsHandler)
	http.HandleFunc("/dashboard/live", dashboardLiveHandler)

	fmt.Println("🚀 Server starting on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println("Server failed:", err)
	}
}
