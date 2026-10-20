use axum::{
    extract::State,
    http::StatusCode,
    response::IntoResponse,
    routing::{get, post},
    Json, Router,
};
use serde::{Deserialize, Serialize};
use std::{env, net::SocketAddr, sync::Arc};
use tracing::{error, info};

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct WarReport {
    pub country: String,
    pub warplanes_in_air: i32,
    pub warships_in_water: i32,
    pub timestamp: String,
}

#[derive(Clone)]
struct AppState {
    client: reqwest::Client,
    go_target_url: String,
}

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| "info".into()),
        )
        .init();

    let port: u16 = env::var("PORT")
        .unwrap_or_else(|_| "8080".to_string())
        .parse()
        .unwrap_or(8080);

    let go_target_url = env::var("GO_TARGET_URL")
        .unwrap_or_else(|_| "http://go-deployment1-service:8081/report".to_string());

    let state = Arc::new(AppState {
        client: reqwest::Client::new(),
        go_target_url,
    });

    // Rutas: Health check y endpoints de reporte (/report y /grpc-202304063)
    let app = Router::new()
        .route("/healthz", get(healthz_handler))
        .route("/report", post(handle_report))
        .route("/grpc-202304063", post(handle_report))
        .with_state(state);

    let addr = SocketAddr::from(([0, 0, 0, 0], port));
    info!("Iniciando API REST en Rust en http://{}", addr);

    let listener = tokio::net::TcpListener::bind(addr)
        .await
        .expect("No se pudo iniciar el listener TCP");

    axum::serve(listener, app)
        .await
        .expect("Fallo en el servidor Axum");
}

async fn healthz_handler() -> impl IntoResponse {
    (StatusCode::OK, "OK")
}

async fn handle_report(
    State(state): State<Arc<AppState>>,
    Json(payload): Json<WarReport>,
) -> impl IntoResponse {
    // Validar país permitido
    let country_upper = payload.country.to_uppercase();
    let valid_countries = ["USA", "RUS", "CHN", "ESP", "GTM"];

    if !valid_countries.contains(&country_upper.as_str()) {
        return (
            StatusCode::BAD_REQUEST,
            Json(serde_json::json!({
                "error": "País no válido. Permitidos: USA, RUS, CHN, ESP, GTM",
                "received": payload.country
            })),
        );
    }

    let forward_payload = WarReport {
        country: country_upper,
        ..payload
    };

    // Reenviar al Deployment 1 de Go
    match state
        .client
        .post(&state.go_target_url)
        .json(&forward_payload)
        .send()
        .await
    {
        Ok(resp) => {
            if resp.status().is_success() {
                (
                    StatusCode::OK,
                    Json(serde_json::json!({
                        "status": "success",
                        "message": "Reporte recibido y reenviado a Go Deployment 1"
                    })),
                )
            } else {
                error!("Error recibido desde Go Deployment 1: HTTP {}", resp.status());
                (
                    StatusCode::BAD_GATEWAY,
                    Json(serde_json::json!({
                        "status": "error",
                        "message": "Error en Go Deployment 1"
                    })),
                )
            }
        }
        Err(err) => {
            error!("Fallo de conexión hacia Go Deployment 1: {}", err);
            (
                StatusCode::SERVICE_UNAVAILABLE,
                Json(serde_json::json!({
                    "status": "error",
                    "message": "No se pudo contactar a Go Deployment 1"
                })),
            )
        }
    }
}
