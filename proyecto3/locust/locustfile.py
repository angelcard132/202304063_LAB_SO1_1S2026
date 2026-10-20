import random
from datetime import datetime, timezone
from locust import HttpUser, task, between

# Países aceptados por el sistema según especificación
COUNTRIES = ["USA", "RUS", "CHN", "ESP", "GTM"]

# Carnet: 202304063 (Ruta Gateway API: /grpc-202304063)
CARNET = "202304063"
ENDPOINT = f"/grpc-{CARNET}"

class MilitaryReportsUser(HttpUser):
    # Tiempo de espera entre peticiones
    wait_time = between(0.05, 0.2)

    @task
    def send_war_report(self):
        # Rangos sugeridos:
        # warplanes_in_air: entre 0 y 50
        # warships_in_water: entre 0 y 30
        payload = {
            "country": random.choice(COUNTRIES),
            "warplanes_in_air": random.randint(0, 50),
            "warships_in_water": random.randint(0, 30),
            "timestamp": datetime.now(timezone.utc).isoformat()
        }

        headers = {"Content-Type": "application/json"}
        with self.client.post(ENDPOINT, json=payload, headers=headers, catch_response=True) as response:
            if response.status_code == 200:
                response.success()
            else:
                response.failure(f"Fallo HTTP {response.status_code}: {response.text}")
