# IP Allocator API

A robust Go-based REST API for hierarchical IP address allocation supporting both IPv4 and IPv6 networks. The API allows for dynamic IP allocation based on region → zone → sub-zone hierarchy with support for preferred IP addresses and automatic allocation from available pools.

## Features

- **Hierarchical IP Management**: Organize IP ranges using region → zone → sub-zone structure
- **Dual Stack Support**: Full IPv4 and IPv6 support with CIDR block management
- **Intelligent Allocation**: Preferred IP allocation with automatic fallback to available IPs
- **MongoDB Integration**: Scalable document-based storage with efficient hierarchy traversal
- **REST API**: Clean, well-documented RESTful endpoints
- **Validation**: Comprehensive input validation and error handling
- **Docker Support**: Containerized deployment with MongoDB
- **Graceful Shutdown**: Proper connection management and cleanup

## Architecture

```
ip-allocator-api/
├── cmd/api/               # Application entry point
├── internal/
│   ├── config/           # Configuration management
│   ├── models/           # Data structures and MongoDB models
│   ├── handlers/         # HTTP request handlers
│   ├── services/         # Business logic layer
│   ├── database/         # Database connection management
│   └── utils/            # Utility functions for IP handling
├── api/                  # Route definitions and middleware
├── configs/              # Configuration files
├── Dockerfile           # Container definition
├── docker-compose.yml   # Multi-container setup
├── Makefile            # Build and development commands
└── README.md           # This file
```

## Prerequisites

- **Go 1.21+**: Download from [golang.org](https://golang.org/dl/)
- **MongoDB**: Either local installation or Docker
- **Docker & Docker Compose** (optional): For containerized deployment
- **VS Code**: Recommended IDE with Go extension

## Quick Start

### Option 1: Local Development

1. **Clone and Setup**
   ```bash
   mkdir ip-allocator-api
   cd ip-allocator-api
   # Copy all provided files to this directory
   ```

2. **Install Dependencies**
   ```bash
   make deps
   ```

3. **Start MongoDB** (if not using Docker)
   ```bash
   # Windows
   mongod --dbpath C:\data\db

   # macOS/Linux
   sudo systemctl start mongod
   ```

4. **Configure Environment**
   ```bash
   # Edit configs/app.env with your MongoDB connection details
   DB_URI=mongodb://localhost:27017
   DB_NAME=ip_allocator
   SERVER_HOST=0.0.0.0
   SERVER_PORT=8080
   ENV=development
   ```

5. **Run the Application**
   ```bash
   make run
   ```

### Option 2: Docker Deployment

1. **Setup Project**
   ```bash
   mkdir ip-allocator-api
   cd ip-allocator-api
   # Copy all provided files to this directory
   ```

2. **Start with Docker Compose**
   ```bash
   make docker-run
   ```

   This will:
   - Start MongoDB container
   - Build and start the API container
   - Configure networking between containers

## File Structure Explanation

### Core Application Files

| File/Directory | Purpose | Description |
|----------------|---------|-------------|
| `cmd/api/main.go` | Entry Point | Application startup, configuration loading, graceful shutdown |
| `internal/config/config.go` | Configuration | Viper-based config management from files and environment |
| `internal/models/` | Data Models | MongoDB document structures and request/response models |
| `internal/handlers/` | HTTP Handlers | Request processing, validation, and response formatting |
| `internal/services/` | Business Logic | IP allocation algorithms and database operations |
| `internal/database/` | Database Layer | MongoDB connection management and utilities |
| `internal/utils/` | Utilities | IP address manipulation and HTTP response helpers |
| `api/routes.go` | Route Definition | HTTP endpoints, middleware, and CORS configuration |

### Configuration Files

| File | Purpose |
|------|---------|
| `go.mod` | Go module definition and dependencies |
| `configs/app.env` | Environment-specific configuration |
| `Dockerfile` | Container image definition |
| `docker-compose.yml` | Multi-container orchestration |
| `Makefile` | Build automation and developer commands |

## API Endpoints

### Health Check
```http
GET /api/v1/health
```

### Region Management
```http
# Get all regions
GET /api/v1/regions

# Create new region
POST /api/v1/regions

# Get specific region hierarchy
GET /api/v1/regions/{region}

# Get sub-zone details
GET /api/v1/regions/{region}/zones/{zone}/subzones/{subzone}
```

### IP Allocation
```http
# Allocate IP addresses
POST /api/v1/allocate
```

## Usage Examples

### 1. Create Region Structure

```bash
curl -X POST http://localhost:8080/api/v1/regions \
  -H "Content-Type: application/json" \
  -d '{
    "name": "us-west",
    "zones": [
      {
        "name": "zone-a",
        "sub_zones": [
          {
            "name": "web-tier",
            "ipv4_cidr": "10.0.1.0/24",
            "ipv6_cidr": "2001:db8::/64"
          },
          {
            "name": "app-tier",
            "ipv4_cidr": "10.0.2.0/24",
            "ipv6_cidr": "2001:db8:1::/64"
          }
        ]
      }
    ]
  }'
```

### 2. Allocate IP Addresses

```bash
# Allocate single IPv4 address
curl -X POST http://localhost:8080/api/v1/allocate \
  -H "Content-Type: application/json" \
  -d '{
    "region": "us-west",
    "zone": "zone-a",
    "sub_zone": "web-tier",
    "ip_version": "ipv4",
    "count": 1
  }'

# Allocate with preferred IPs
curl -X POST http://localhost:8080/api/v1/allocate \
  -H "Content-Type: application/json" \
  -d '{
    "region": "us-west",
    "zone": "zone-a",
    "sub_zone": "web-tier",
    "ip_version": "ipv4",
    "count": 2,
    "preferred_ips": ["10.0.1.10", "10.0.1.11"]
  }'

# Allocate both IPv4 and IPv6
curl -X POST http://localhost:8080/api/v1/allocate \
  -H "Content-Type: application/json" \
  -d '{
    "region": "us-west",
    "zone": "zone-a",
    "sub_zone": "web-tier",
    "ip_version": "both",
    "count": 4
  }'
```

### 3. Query Region Information

```bash
# Get all regions
curl http://localhost:8080/api/v1/regions

# Get specific region
curl http://localhost:8080/api/v1/regions/us-west

# Get sub-zone details
curl http://localhost:8080/api/v1/regions/us-west/zones/zone-a/subzones/web-tier
```

## Development Commands

```bash
# Install dependencies
make deps

# Run application
make run

# Run with hot reload (requires air)
make dev

# Run tests
make test

# Format code
make fmt

# Build binary
make build

# Clean build files
make clean

# Docker operations
make docker-build
make docker-run
make docker-stop
make docker-clean
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_URI` | MongoDB connection string | `mongodb://localhost:27017` |
| `DB_NAME` | Database name | `ip_allocator` |
| `SERVER_HOST` | Server bind address | `0.0.0.0` |
| `SERVER_PORT` | Server port | `8080` |
| `ENV` | Environment (development/production) | `development` |

### MongoDB Setup

The application requires a MongoDB instance. The default configuration expects:
- Host: `localhost:27017`
- Database: `ip_allocator`
- Collections: `regions` (auto-created)

For production, consider:
- Authentication enabled
- Replica set configuration
- Appropriate indexing for performance

## Data Model

### Region Structure
```json
{
  "_id": "ObjectId",
  "name": "us-west",
  "zones": [
    {
      "name": "zone-a",
      "sub_zones": [
        {
          "name": "web-tier",
          "ipv4_cidr": "10.0.1.0/24",
          "ipv6_cidr": "2001:db8::/64",
          "allocated_ipv4": ["10.0.1.1", "10.0.1.2"],
          "allocated_ipv6": ["2001:db8::1"],
          "reserved_ipv4": ["10.0.1.254"],
          "reserved_ipv6": []
        }
      ]
    }
  ],
  "created_at": "2025-01-01T00:00:00Z",
  "updated_at": "2025-01-01T00:00:00Z"
}
```

## Error Handling

The API provides consistent error responses:

```json
{
  "success": false,
  "error": "validation_error",
  "message": "Invalid IP version. Must be 'ipv4', 'ipv6', or 'both'",
  "timestamp": "2025-01-01T00:00:00Z"
}
```

Common error types:
- `bad_request`: Invalid input data
- `validation_error`: Schema validation failures
- `not_found`: Resource not found
- `conflict`: Duplicate resource creation
- `internal_server_error`: Server-side errors

## Testing

### Manual Testing
Use the provided curl examples or tools like Postman to test the API endpoints.

### Automated Testing
```bash
make test
```

### Health Check
```bash
curl http://localhost:8080/api/v1/health
```

Expected response:
```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "timestamp": "2025-01-01T00:00:00Z",
    "service": "IP Allocator API",
    "version": "1.0.0"
  }
}
```

## Performance Considerations

1. **Indexing**: Consider adding indexes on region/zone/sub-zone names for faster queries
2. **Connection Pooling**: MongoDB Go driver handles connection pooling automatically
3. **Concurrency**: The service handles concurrent requests safely with proper context management
4. **Memory**: IP range calculations are optimized for large CIDR blocks

## Security Notes

1. **Input Validation**: All inputs are validated using struct tags and custom validators
2. **CORS**: Configure allowed origins in production
3. **Authentication**: Consider adding JWT or API key authentication for production use
4. **Rate Limiting**: Implement rate limiting for production deployments

## Troubleshooting

### Common Issues

1. **MongoDB Connection Failed**
   - Verify MongoDB is running
   - Check connection string in `configs/app.env`
   - Ensure network connectivity

2. **Port Already in Use**
   - Change `SERVER_PORT` in configuration
   - Kill process using the port: `lsof -ti:8080 | xargs kill`

3. **Invalid CIDR Blocks**
   - Ensure CIDR notation is correct (e.g., `10.0.1.0/24`)
   - Verify IPv4/IPv6 format consistency

4. **Build Errors**
   - Run `go mod tidy` to resolve dependencies
   - Ensure Go 1.21+ is installed

### Logs

Application logs include:
- HTTP request logging with timing
- Database operation results
- Error details with stack traces
- Graceful shutdown events

## Production Deployment

For production deployment:

1. **Environment Configuration**
   ```bash
   ENV=production
   DB_URI=mongodb://username:password@mongodb-host:27017/ip_allocator
   ```

2. **Security Hardening**
   - Use HTTPS with proper certificates
   - Implement authentication middleware
   - Configure specific CORS origins
   - Add rate limiting

3. **Monitoring**
   - Add health check endpoints to load balancer
   - Monitor MongoDB performance
   - Set up logging aggregation

4. **Scaling**
   - Use multiple API instances behind load balancer
   - MongoDB replica set for high availability
   - Consider caching for read-heavy workloads

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass
6. Submit a pull request

## Support

For issues and questions:
1. Check the troubleshooting section
2. Review existing GitHub issues
3. Create a new issue with detailed information
