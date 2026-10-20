use axum::{
    extract::State,
    http::StatusCode,
    routing::{get, post},
    Json, Router,
};
use serde::{Deserialize, Serialize};
use std::{
    env,
    sync::{
        atomic::{AtomicU64, Ordering},
        Arc,
    },
};

// Contador de reportes procesados por este pod (se imprime en los logs)
static CONTADOR: AtomicU64 = AtomicU64::new(0);

// La forma del JSON que esperamos (la misma del enunciado)
#[derive(Debug, Deserialize, Serialize)]
struct WarReport {
    country: String,
    warplanes_in_air: i32,
    warships_in_water: i32,
    timestamp: String,
}

// Estado compartido por todas las peticiones
struct AppState {
    client: reqwest::Client,
    go_url: String,
}

#[tokio::main]
async fn main() {
    let go_url = env::var("GO_API_URL")
        .unwrap_or_else(|_| "http://go-api:8080/report".to_string());
    let port = env::var("PORT").unwrap_or_else(|_| "8000".to_string());

    let state = Arc::new(AppState {
        client: reqwest::Client::new(),
        go_url,
    });

    let app = Router::new()
        .route("/health", get(health))
        .route("/report", post(report))
        .with_state(state);

    let listener = tokio::net::TcpListener::bind(format!("0.0.0.0:{port}"))
        .await
        .unwrap();
    println!("rust-api escuchando en el puerto {port}");
    axum::serve(listener, app).await.unwrap();
}

async fn health() -> &'static str {
    "ok"
}

async fn report(
    State(state): State<Arc<AppState>>,
    Json(r): Json<WarReport>,
) -> (StatusCode, String) {
    if r.country.len() != 3 {
        println!("[rust-api] rechazado: country invalido '{}'", r.country);
        return (StatusCode::BAD_REQUEST, "country debe tener 3 letras".into());
    }

    let n = CONTADOR.fetch_add(1, Ordering::Relaxed) + 1;

    match state.client.post(&state.go_url).json(&r).send().await {
        Ok(resp) if resp.status().is_success() => {
            println!(
                "[rust-api] #{} {} aviones={} barcos={} -> go-api OK",
                n, r.country, r.warplanes_in_air, r.warships_in_water
            );
            (StatusCode::OK, r#"{"status":"forwarded"}"#.into())
        }
        Ok(resp) => {
            println!(
                "[rust-api] #{} {} -> go-api respondio {}",
                n, r.country, resp.status()
            );
            (
                StatusCode::BAD_GATEWAY,
                format!("el servicio Go respondió {}", resp.status()),
            )
        }
        Err(e) => {
            println!(
                "[rust-api] #{} {} -> error contactando a go-api: {}",
                n, r.country, e
            );
            (
                StatusCode::BAD_GATEWAY,
                format!("no se pudo contactar al servicio Go: {e}"),
            )
        }
    }
}