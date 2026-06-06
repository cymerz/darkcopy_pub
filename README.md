# DarkCopy

**DarkCopy** is a self-hosted, secure text paste and file hosting platform. It is built with a decoupled architecture featuring a Go API backend and a responsive Next.js frontend dashboard utilizing a glassmorphism design.

---

## Features

*   **Secure Paste Bin**: Share code snippets and text notes with customizable syntax highlighting powered by Chroma.
*   **File Sharing**: Upload and download files up to 100 MB with automatic relative unit formatting and responsive visual grids.
*   **Flexible Storage Options**: Support for AWS S3, Cloudflare R2, MinIO, or fallback to the local filesystem if external S3 storage is not configured.
*   **Automated Expiry Cleanup**: Integrated background worker that automatically deletes expired pastes and files from both the database and storage.
*   **Access Control**: Secure sharing with optional password protection for pastes and files, hashed using bcrypt.
*   **Admin Dashboard**: Manage active pastes, files, abuse reports, and adjust application quotas.
*   **Docker Integration**: Build and deploy the entire stack (PostgreSQL, Backend, Frontend) using Docker Compose.

---

## Repository Structure

This repository is organized as a monorepo:

```text
├── backend/               # Go API server, DB migrations, and static templates
│   ├── cmd/server/        # Main server entrypoint
│   ├── internal/          # Business logic (access, admin, DB, expiry, file, handler, etc.)
│   └── Dockerfile         # Multi-stage production build for Go backend
├── frontend/              # Next.js Web app (with glassmorphism theme)
│   ├── app/               # Next.js App Router (pages & Server Components)
│   ├── components/        # Reusable UI elements (FileList, PasteList, FileUploader, etc.)
│   └── Dockerfile         # Production Dockerfile for Node runtime
├── docker-compose.yml     # Multi-container orchestration (Db + Backend + Frontend)
└── .gitignore             # Root git ignore file to exclude configuration and credential files
```

---

## Quick Start with Docker Compose

Ensure you have **Docker** and **Docker Compose** installed on your server or local machine.

### 1. Build and Launch the Application

In the repository root, run:
```bash
docker compose up --build
```
This command will:
1. Start a PostgreSQL 15 database, initialize data schemas, and wait until it is healthy.
2. Build the Go API binary and start serving on port `8080`.
3. Compile the Next.js production build and start serving on port `3000`.

### 2. Access the Application
- **Frontend Dashboard**: Open `http://localhost:3000` in your browser.
- **Go API Backend**: Health check available at `http://localhost:8080`.

---

## Environment Configurations

The application supports configuration via environment variables:

- **Docker Compose Setup**: Copy the `.env.example` in the root directory to `.env` to customize database credentials, admin tokens, and S3 settings.
- **Local Development Setup**: Create a `.env` file in the `backend/` directory based on `backend/.env.example` or set the variables in your environment.

### Backend Configurations (`backend/.env` or root `.env`)

| Variable | Description | Default |
| :--- | :--- | :--- |
| `PORT` | Port to run the Go server | `8080` |
| `DATABASE_URL` | PostgreSQL connection string | *Required* |
| `UPLOAD_DIR` | Local uploads directory (if not using S3) | `./uploads` |
| `CLEANUP_INTERVAL` | Expiry cleanup worker cycle (e.g., `5m`, `1h`) | `5m` |
| `ADMIN_TOKEN` | Secret key to authorize admin API | *Disabled if empty* |
| `S3_BUCKET` | Optional S3 bucket name to activate S3 Storage | *Falls back to local* |
| `S3_REGION` | S3 region | `us-east-1` |
| `S3_ENDPOINT` | Custom endpoint (useful for MinIO/R2) | *Standard AWS S3* |
| `S3_ACCESS_KEY` | S3 API Access Key | *Static credential* |
| `S3_SECRET_KEY` | S3 API Secret Key | *Static credential* |

---

## Local Development Setup

If you prefer to run services natively for development:

### Prerequisites
- **Go** (version 1.26 or higher)
- **Node.js** (version 20 or higher)
- **PostgreSQL**

### 1. Running Go API Backend

1. Navigate to the backend directory:
   ```bash
   cd backend
   ```
2. Copy the sample environment file:
   ```bash
   cp .env.example .env
   # Edit .env and configure your local DATABASE_URL
   ```
3. Run the Go tests to verify integrity:
   ```bash
   go test ./...
   ```
4. Start the server:
   ```bash
   go run ./cmd/server
   ```

### 2. Running Next.js Frontend

1. Navigate to the frontend directory:
   ```bash
   cd ../frontend
   ```
2. Install npm packages:
   ```bash
   npm install
   ```
3. Run in development mode:
   ```bash
   npm run dev
   ```
4. Open `http://localhost:3000` in your browser. All API requests to `/api/*` are proxied to the backend at `http://localhost:8080` using Next.js rewrites.

---

## Security Architecture & Design

- **Password Hashing**: Client-defined passwords for protected pastes and uploads are salted and hashed using bcrypt.
- **Expiry Cleanup**: The background daemon deletes files from both the database and S3/local storage, ensuring no orphaned data remains.
- **Optimized Container Images**: The Go backend Docker image is built using a secure multi-stage pipeline, outputting a minimal container image (~18MB) to reduce the attack surface.

---

## License

This project is open-source and available under the [MIT License](LICENSE).
