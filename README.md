# Phone Number to Telegram Proxy

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

A service that allows sending messages to Telegram users by specifying their phone numbers.

## Table of Contents
- [Phone Number to Telegram Proxy](#phone-number-to-telegram-proxy)
  - [Table of Contents](#table-of-contents)
  - [Features](#features)
  - [Installation](#installation)
    - [Prerequisites](#prerequisites)
    - [Using Pre-built Binaries (Recommended)](#using-pre-built-binaries-recommended)
    - [Using Docker (Recommended)](#using-docker-recommended)
    - [From Source (Alternative)](#from-source-alternative)
  - [Configuration](#configuration)
    - [Environment Variables](#environment-variables)
    - [Configuration File](#configuration-file)
  - [Usage](#usage)
    - [Using the API](#using-the-api)
    - [Using the Telegram Bot](#using-the-telegram-bot)
  - [API Documentation](#api-documentation)
    - [Endpoints](#endpoints)
      - [Send Message](#send-message)
  - [Contributing](#contributing)
  - [License](#license)


## Features

- **Phone Number to Telegram Proxy**: Send messages to Telegram users by specifying their phone number
- **State Management**: Finite State Machine (FSM) for handling bot interactions
- **Secure Storage**: Phone numbers are hashed for security
- **Modular Architecture**: Well-structured components for maintainability
- **Dependency Injection**: Using Uber FX for better testability

## Installation

### Prerequisites

- Redis server
- Telegram Bot Token

### Using Pre-built Binaries (Recommended)

1. Go to the [GitHub Releases](https://github.com/capcom6/phone2tg-proxy/releases) page
2. Download the pre-built binary for your operating system and architecture
3. Make the binary executable:
   ```bash
   chmod +x phone2tg-proxy-linux-amd64
   ```
4. Run the binary:
   ```bash
   ./phone2tg-proxy-linux-amd64
   ```

### Using Docker (Recommended)

1. Pull the latest image:
   ```bash
   docker pull ghcr.io/capcom6/phone2tg-proxy:latest
   ```
2. Run the container:
   ```bash
   docker run -d \
     --env TELEGRAM__TOKEN=your_telegram_bot_token \
     --env REDIS__URL=redis://localhost:6379/0 \
     --env STORAGE__SECRET=your_storage_secret \
     --publish 3000:3000 \
     --name phone2tg-proxy \
     ghcr.io/capcom6/phone2tg-proxy:latest
   ```

### From Source (Alternative)

1. Clone the repository:
   ```bash
   git clone https://github.com/capcom6/phone2tg-proxy.git
   cd phone2tg-proxy
   ```

2. Download dependencies:
   ```bash
   go mod download
   ```

3. Build and run:
   ```bash
   go run main.go
   ```

## Configuration

The service can be configured using environment variables or a configuration file.

### Environment Variables

| Variable             | Description                     | Default                    |
| -------------------- | ------------------------------- | -------------------------- |
| `HTTP__ADDRESS`      | HTTP server address             | `127.0.0.1:3000`           |
| `HTTP__PROXY_HEADER` | HTTP proxy header               | `X-Forwarded-For`          |
| `HTTP__PROXIES`      | HTTP trusted proxies            | empty                      |
| `TELEGRAM__TOKEN`    | Your Telegram bot token         | Required                   |
| `REDIS__URL`         | Redis connection URL            | `redis://localhost:6379/0` |
| `STORAGE__SECRET`    | Secret for phone number hashing | Required                   |

### Configuration File

Create a `.env` file in the project root with your configuration:

```env
TELEGRAM__TOKEN=your_telegram_bot_token
REDIS__URL=redis://localhost:6379/0
STORAGE__SECRET=your_storage_secret
```

## Usage

### Using the API

The service provides a REST API for sending messages. See the [API Documentation](#api-documentation) for details.

### Using the Telegram Bot

1. Start a chat with your bot
2. Send `/start` to register your phone number
3. Send your contact to associate your phone number with your Telegram account
4. You can now use this bot to send messages to registered users by providing their phone numbers.

## API Documentation

The service provides a REST API for sending messages.

### Endpoints

#### Send Message

- **URL**: `/api/v1/messages`
- **Method**: `POST`
- **Description**: Send a message to a user by phone number. The phoneNumber must be in E.164 format. Validation failures will occur if the format is not followed.
- **Request Body**:
  ```json
  {
    "phoneNumber": "string (E.164 format, e.g., +1234567890)",
    "text": "string"
  }
  ```
- **Response**:
  ```json
  {
    "id": "integer"
  }
  ```
- **Error Responses**:
  - `400 Bad Request`: Invalid request format
  - `404 Not Found`: Phone number not found
  - `500 Internal Server Error`: Server error

## Contributing

Contributions are welcome! Please follow these steps:

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## License

This project is licensed under the Apache 2.0 License - see the [LICENSE](LICENSE) file for details.
