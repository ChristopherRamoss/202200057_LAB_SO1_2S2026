use axum::{
    extract::State,
    http::StatusCode,
    routing::{get, post},
    Json, Router,
};
use serde::{Deserialize, Serialize};
use std::{env, sync::Arc};

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
    // Configuración por variables de entorno, con valores por defecto
    let go_url = env::var("GO_API_URL")
        .unwrap_or_else(|_| "http://go-api:8080/report".to_string());
    let port = env::var("PORT").unwrap_or_else(|_| "8000".to_string());

    // Un solo cliente HTTP reutilizado (mantiene conexiones abiertas)
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

// Kubernetes lo usará para saber si el pod está vivo
async fn health() -> &'static str {
    "ok"
}

async fn report(
    State(state): State<Arc<AppState>>,
    Json(r): Json<WarReport>,
) -> (StatusCode, String) {
    // Validación básica: el país debe ser un código de 3 letras
    if r.country.len() != 3 {
        return (StatusCode::BAD_REQUEST, "country debe tener 3 letras".into());
    }

    // Reenviamos el reporte al servicio Go
    match state.client.post(&state.go_url).json(&r).send().await {
        Ok(resp) if resp.status().is_success() => {
            (StatusCode::OK, r#"{"status":"forwarded"}"#.into())
        }
        Ok(resp) => (
            StatusCode::BAD_GATEWAY,
            format!("el servicio Go respondió {}", resp.status()),
        ),
        Err(e) => (
            StatusCode::BAD_GATEWAY,
            format!("no se pudo contactar al servicio Go: {e}"),
        ),
    }
}