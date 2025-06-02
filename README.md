# 🌈 A2A‑Go

> _"Combine A2A and MCP to create advanced, distributed agentic systems!"_

A2A-Go is a framework for building scalable, distributed agentic AI systems. It promotes a microservice architecture where agents and tools operate as independent services, deployable locally or across a network. The framework implements the Agent-to-Agent (A2A) protocol and utilizes the Model Context Protocol (MCP) for standardized tool interaction and data exchange.

[![Go CI/CD](https://github.com/theapemachine/a2a-go/actions/workflows/main.yml/badge.svg)](https://github.com/theapemachine/a2a-go/actions/workflows/main.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/theapemachine/a2a-go)](https://goreportcard.com/report/github.com/theapemachine/a2a-go)
[![GoDoc](https://godoc.org/github.com/theapemachine/a2a-go?status.svg)](https://godoc.org/github.com/theapemachine/a2a-go)
[![License: UNLICENSE](https://img.shields.io/badge/License-UNLICENSE-green.svg)](https://opensource.org/licenses/MIT)
[![Bugs](https://sonarcloud.io/api/project_badges/measure?project=TheApeMachine_a2a-go&metric=bugs)](https://sonarcloud.io/summary/new_code?id=TheApeMachine_a2a-go)
[![Code Smells](https://sonarcloud.io/api/project_badges/measure?project=TheApeMachine_a2a-go&metric=code_smells)](https://sonarcloud.io/summary/new_code?id=TheApeMachine_a2a-go)
[![Duplicated Lines (%)](https://sonarcloud.io/api/project_badges/measure?project=TheApeMachine_a2a-go&metric=duplicated_lines_density)](https://sonarcloud.io/summary/new_code?id=TheApeMachine_a2a-go)
[![Lines of Code](https://sonarcloud.io/api/project_badges/measure?project=TheApeMachine_a2a-go&metric=ncloc)](https://sonarcloud.io/summary/new_code?id=TheApeMachine_a2a-go)
[![Reliability Rating](https://sonarcloud.io/api/project_badges/measure?project=TheApeMachine_a2a-go&metric=reliability_rating)](https://sonarcloud.io/summary/new_code?id=TheApeMachine_a2a-go)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=TheApeMachine_a2a-go&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=TheApeMachine_a2a-go)
[![Technical Debt](https://sonarcloud.io/api/project_badges/measure?project=TheApeMachine_a2a-go&metric=sqale_index)](https://sonarcloud.io/summary/new_code?id=TheApeMachine_a2a-go)
[![Maintainability Rating](https://sonarcloud.io/api/project_badges/measure?project=TheApeMachine_a2a-go&metric=sqale_rating)](https://sonarcloud.io/summary/new_code?id=TheApeMachine_a2a-go)
[![Vulnerabilities](https://sonarcloud.io/api/project_badges/measure?project=TheApeMachine_a2a-go&metric=vulnerabilities)](https://sonarcloud.io/summary/new_code?id=TheApeMachine_a2a-go)

![A2A‑Go](a2a-go.png)

**a2a‑go** is a reference Go implementation of the [**Agent‑to‑Agent (A2A)**
protocol](https://google.github.io/A2A/#/) by Google, including the proposed
interoperability with the [**Model Context Protocol (MCP)**](https://modelcontextprotocol.io).

> 🚧 **Work in progress** 🚧 Consider this project a proof of concept at best, and subject
> to sudden changes.

## ✨ Features

- [x] **Agent‑to‑Agent (A2A)** protocol implementation

  - [x] _Send Task_ to send a new task to an agent
  - [x] _Get Task_ to retrieve a task by ID
  - [x] _Cancel Task_ to cancel a task
  - [x] _Stream Task_ to stream the task results
  - [x] _Set Push Notification_ to configure push notifications for a task
  - [x] _Get Push Notification_ to retrieve the push notification configuration for a task
  - [x] _Structured Outputs_ to return structured data from an agent
  - [x] _Fine‑tuning_ to fine‑tune an agent on a dataset
  - [x] _Image Generation_ to generate images with an agent
  - [x] _Audio Transcription_ to transcribe audio
  - [x] _Text‑to‑Speech_ to convert text to speech

- [x] **Model Context Protocol (MCP)** interoperability

  - [x] _Tool Calling_ to call tools and receive the results
  - [x] _List Prompts_ to retrieve a list of prompts from an agent
  - [x] _Get Prompt_ to retrieve a prompt by ID
  - [x] _Set Prompt_ to create or update a prompt
  - [x] _Delete Prompt_ to delete a prompt by ID
  - [ ] _List Resources_ to retrieve a list of resources from an agent
  - [ ] _Get Resource_ to retrieve a resource by ID
  - [ ] _Set Resource_ to create or update a resource
  - [ ] _Delete Resource_ to delete a resource by ID
  - [x] _Sampling_ to sample a task from an agent
  - [x] _Roots_ to get the root task for a task

- [x] **Built‑in tools**

  - [x] _Browser_ to browse the web
  - [x] _Docker_ to run Docker commands
  - [x] _GitHub_ to search GitHub
  - [x] _Memory_ to store and retrieve memories
  - [x] _Qdrant_ to store and retrieve vectors
  - [x] _Neo4j_ to store and retrieve graph data

---

## 🚀 Quick Start

Use the `Makefile` to run a full containerized distributed system, demonstrating A2A and MCP interoperability.

```bash
make server
make client
```

The `ui` agent is mapped to port `3212`. If you run the TUI outside the Docker
network, update `agent.ui.url` in `~/.a2a-go/config.yml` to point to
`http://localhost:3212`.
