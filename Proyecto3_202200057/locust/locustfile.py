import json
import os
import random
from datetime import datetime, timezone

from locust import HttpUser, task, between

# Archivo de entrada, descargado desde Zot como OCI Artifact
CONFIG_PATH = os.path.join(os.path.dirname(__file__), "config-reportes.json")
with open(CONFIG_PATH) as f:
    CONFIG = json.load(f)

COUNTRIES = CONFIG["countries"]
PLANES = CONFIG["warplanes_in_air"]
SHIPS = CONFIG["warships_in_water"]


class WarReportUser(HttpUser):
    host = "http://localhost"
    wait_time = between(0.1, 0.5)

    @task
    def send_report(self):
        payload = {
            "country": random.choice(COUNTRIES),
            "warplanes_in_air": random.randint(PLANES["min"], PLANES["max"]),
            "warships_in_water": random.randint(SHIPS["min"], SHIPS["max"]),
            "timestamp": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        }
        self.client.post("/grpc-202200057", json=payload)