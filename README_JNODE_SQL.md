# gossiped jnode SQL Database Integration

This document describes how to use gossiped with jnode's SQL database instead of file-based message areas.

## Overview

The jnode SQL integration allows gossiped to read and write messages directly from/to a jnode database. This provides several advantages:

- **Multi-user support**: Multiple applications can access the same message base
- **Better performance**: Database indexing and optimization
- **Data integrity**: ACID compliance and transaction support
- **Advanced features**: Complex queries, statistics, and reporting
- **Centralized storage**: All FTN data in one database

## Supported Databases

- **MySQL** - Production-ready, high performance
- **PostgreSQL** - Advanced features, excellent reliability 
- **SQLite** - Simple setup, good for single-user or testing

## Configuration

### Basic Setup

1. **Copy example configuration:**
   ```bash
   cp gossiped.jnode.example.yml gossiped.yml
   ```

2. **Configure database connection:**
   ```yaml
   database:
     driver: "mysql"  # or "postgres", "sqlite"
     dsn: "jnode:password@tcp(localhost:3306)/jnode?charset=utf8mb4&parseTime=True&loc=Local"
     max_open_conns: 25
     max_idle_conns: 5
     conn_max_lifetime: "5m"

   areafile:
     type: "jnode-sql"
     path: ""  # Not used for SQL areas
   ```

### Database Connection Strings (DSN)

#### MySQL:
```
jnode:password@tcp(localhost:3306)/jnode?charset=utf8mb4&parseTime=True&loc=Local
```

#### PostgreSQL:
```
host=localhost user=jnode password=pass dbname=jnode port=5432 sslmode=disable TimeZone=UTC
```

### Configuration Options

- **driver**: Database type (`mysql`, `postgres`)
- **dsn**: Database connection string
- **max_open_conns**: Maximum open database connections (default: 25)
- **max_idle_conns**: Maximum idle connections (default: 5)  
- **conn_max_lifetime**: Connection maximum lifetime (default: 5m)

## Testing Database Connection

Use the included database test utility:

```bash
# Build the test utility
go build -o dbtest ./cmd/dbtest

# Test your configuration
./dbtest gossiped.yml
```

The test utility will:
- Validate your configuration
- Test database connectivity
- Display connection statistics
- Show message counts by area type
- Verify database schema

## Running gossiped with SQL

```bash
# Using your SQL configuration
./gossiped gossiped.yml

# The application will:
# 1. Connect to the database
# 2. Load areas from echoarea table
# 3. Display database connection statistics
# 4. Start the normal UI
```

## Database Schema

The integration uses jnode's complete database schema:

### Core Tables:
- **echoarea**: Message area definitions
- **echomail**: Public messages (echomails)
- **netmail**: Private messages (netmails)
- **links**: FTN node configurations
- **subscription**: Area subscriptions per link

### Advanced Tables:
- **routing**: Netmail routing rules
- **schedule**: Script execution scheduling
- **jscripts**: JavaScript code storage
- **filearea/filemail**: File distribution

## Features Supported

### ✅ Fully Implemented:
- Reading messages from database
- Writing new messages to database
- Area listing and navigation
- Message listing with pagination
- Message deletion
- Character set handling
- FTN address parsing
- Netmail and echomail support

### 🔄 Planned/Enhanced:
- Message searching and filtering
- Advanced statistics and reporting
- User preference storage
- Multi-user last-read tracking
- Message routing and forwarding

### Debug Mode:

Enable debug logging by adding to configuration:
```yaml
log_level: debug
```

Or run with environment variable:
```bash
DEBUG=1 ./gossiped gossiped.yml
```

## Security Considerations

- **Database Credentials**: Store in secure configuration files
- **Network Security**: Use SSL/TLS for remote database connections
- **Access Control**: Limit database user permissions to required tables only
- **Backup**: Regular database backups are essential

## Character Set Handling

jnode SQL databases store all message content in UTF-8 format, while traditional FTN message bases store content in various character sets (CP866, etc.). gossiped automatically handles this conversion:

### Configuration
```yaml
# Character set configuration
chrs:
  default: "UTF-8 4"        # Display charset - UTF-8 for modern terminals
  ibmpc: "CP866 2"          # Fallback for IBMPC charset
  jnodedefault: "CP866 2"   # Charset to use in @CHRS kludges for jnode messages
```

### How It Works
1. **Reading from Database**: Messages stored as UTF-8 are converted to display charset
2. **Writing to Database**: Messages are stored as UTF-8 with appropriate @CHRS kludges
3. **Display**: Configured display charset determines terminal output encoding
4. **FTN Compatibility**: @CHRS kludges indicate original message charset for tosser compatibility

### Line Ending Handling

jnode SQL uses Unix-style line endings (`\n`) in database storage, while FTN protocols use carriage returns (`\r`). gossiped automatically converts between formats:

- **Database Storage**: Unix line endings (`\n`)
- **FTN Processing**: Carriage return line endings (`\r`)
- **Cross-Platform**: Works correctly on Windows, Linux, and macOS

## Example Configurations

### MySQL Production Setup:
```yaml
database:
  driver: "mysql"
  dsn: "jnode:SecurePassword123@tcp(dbserver:3306)/jnode?charset=utf8mb4&parseTime=True&loc=Local&tls=true"
  max_open_conns: 50
  max_idle_conns: 10
  conn_max_lifetime: "10m"

# Character set for international messages
chrs:
  default: "UTF-8 4"
  ibmpc: "CP866 2" 
  jnodedefault: "CP866 2"
```

### PostgreSQL Setup:
```yaml
database:
  driver: "postgres"
  dsn: "host=localhost user=jnode password=SecurePass dbname=jnode port=5432 sslmode=require TimeZone=UTC"
  max_open_conns: 30
  max_idle_conns: 5
  conn_max_lifetime: "15m"

chrs:
  default: "UTF-8 4"
  jnodedefault: "UTF-8 4"
```

