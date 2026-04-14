#!/usr/bin/env bash
# Seeds a health-care demo: Ward A group, patient/room profiles,
# two patient devices, one room sensor, and a stored dashboard.
# Assumes `deploy/docker-compose.yml` stack is running and the backend
# is reachable at localhost:8080.
set -euo pipefail

docker exec -i observer-postgres psql -U observer -d observer <<'SQL'
INSERT INTO device_groups (id, tenant_id, name)
VALUES ('a0000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'Ward A')
ON CONFLICT (id) DO NOTHING;

INSERT INTO device_profiles (id, tenant_id, name, default_fields)
VALUES
  ('a0000000-0000-0000-0000-000000000011', '00000000-0000-0000-0000-000000000001', 'Patient Monitor', ARRAY['heart_rate','spo2','body_temp']),
  ('a0000000-0000-0000-0000-000000000012', '00000000-0000-0000-0000-000000000001', 'Room Sensor', ARRAY['room_temp','humidity','air_quality'])
ON CONFLICT (id) DO NOTHING;

INSERT INTO devices (id, tenant_id, name, type, profile_id, group_id)
VALUES
  ('a0000000-0000-0000-0000-000000000101', '00000000-0000-0000-0000-000000000001', 'Patient Alice (bed 101)', 'patient', 'a0000000-0000-0000-0000-000000000011', 'a0000000-0000-0000-0000-000000000001'),
  ('a0000000-0000-0000-0000-000000000102', '00000000-0000-0000-0000-000000000001', 'Patient Bob (bed 102)',   'patient', 'a0000000-0000-0000-0000-000000000011', 'a0000000-0000-0000-0000-000000000001'),
  ('a0000000-0000-0000-0000-000000000201', '00000000-0000-0000-0000-000000000001', 'Ward A room sensor',     'room',    'a0000000-0000-0000-0000-000000000012', 'a0000000-0000-0000-0000-000000000001')
ON CONFLICT (id) DO NOTHING;
SQL

curl -fsS -X POST localhost:8080/api/v1/dashboards -H 'Content-Type: application/json' -d '{
  "name": "Health Care — Ward A",
  "layout": {"widgets": [
    {"id":"alice_hr","type":"value","config":{"device_id":"a0000000-0000-0000-0000-000000000101","field":"heart_rate","unit":"bpm"},"x":0,"y":0,"w":3,"h":2},
    {"id":"alice_sp","type":"value","config":{"device_id":"a0000000-0000-0000-0000-000000000101","field":"spo2","unit":"%"},"x":3,"y":0,"w":3,"h":2},
    {"id":"alice_bt","type":"value","config":{"device_id":"a0000000-0000-0000-0000-000000000101","field":"body_temp","unit":"°C"},"x":6,"y":0,"w":3,"h":2},
    {"id":"alice_ch","type":"chart","config":{"device_id":"a0000000-0000-0000-0000-000000000101"},"x":0,"y":2,"w":9,"h":5},
    {"id":"bob_hr","type":"value","config":{"device_id":"a0000000-0000-0000-0000-000000000102","field":"heart_rate","unit":"bpm"},"x":0,"y":7,"w":3,"h":2},
    {"id":"bob_sp","type":"value","config":{"device_id":"a0000000-0000-0000-0000-000000000102","field":"spo2","unit":"%"},"x":3,"y":7,"w":3,"h":2},
    {"id":"bob_bt","type":"value","config":{"device_id":"a0000000-0000-0000-0000-000000000102","field":"body_temp","unit":"°C"},"x":6,"y":7,"w":3,"h":2},
    {"id":"bob_ch","type":"chart","config":{"device_id":"a0000000-0000-0000-0000-000000000102"},"x":0,"y":9,"w":9,"h":5},
    {"id":"room_rt","type":"value","config":{"device_id":"a0000000-0000-0000-0000-000000000201","field":"room_temp","unit":"°C"},"x":0,"y":14,"w":3,"h":2},
    {"id":"room_rh","type":"value","config":{"device_id":"a0000000-0000-0000-0000-000000000201","field":"humidity","unit":"%"},"x":3,"y":14,"w":3,"h":2},
    {"id":"room_iaq","type":"value","config":{"device_id":"a0000000-0000-0000-0000-000000000201","field":"air_quality","unit":"aqi"},"x":6,"y":14,"w":3,"h":2},
    {"id":"alerts","type":"alerts","config":{},"x":9,"y":0,"w":3,"h":16}
  ]}
}'
echo
echo "healthcare demo seeded"
