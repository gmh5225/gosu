# gosu

**gosu** (go supervisor) is a work-in-progress process supervisor for production microservices, supporting extended features such as application monitoring, logging, and error management. It's designed as an alternative to PM2, offering a familiar yet improved experience for managing your applications, written in Go for enhanced performance and lower resource consumption.

## Features

In addition to the core features of PM2, gosu offers a number of improvements and new features.

- **Lightweight and Fast**: Leveraging the efficiency of Go, gosu is designed for minimal memory footprint and quick execution.
- **Easy Configuration**: Simple and flexible configuration options via `.json`, `.yaml`, `.toml`, `.js` or `.ts` files.
- **Application Monitoring**: Real-time monitoring of application performance and resource usage.
- **Scaling**: Effortlessly scale your applications across multiple instances with automatic load balancing, failover and scaling using Unix domain sockets or Named Pipes.
- **Compatibility**: Fully compatible with Node.js environments, making it easy to migrate from PM2.
- **Logging and Error Management**: Advanced logging features for better debugging and error tracking.
- **Remote admin**: Manage your applications remotely using the gosu CLI.
- **Task queues**: Manage task queues and background jobs with ease.
- **Build aware**: Not only does gosu restart your application when a file changes, it also rebuilds your application if you're using a build tool such as Vite.
- **Cross server messaging**: Easily communicate between applications running on different servers using the gosu IPC channel.
- **Work pipelines**: Easily create pipelines of work to be executed in parallel or in sequence with retry and error handling.

## Installation

```bash
go install github.com/can1357/gosu
```

## Usage

(Documentation TBD)

Start a job:

```bash
gosu launch "run node app.js" --id=app_name --n=2 # Launch 2 instances of the application, no language specific features.
gosu launch "ts ./app.ts" --id=app_name  # Typescript support via autoamtic tsx/ts-node discovery in NPM repository.
gosu launch "@./config.js" --id=myserver # Creates a job named myserver as specified in the config file.
gosu launch "..." --launch="boot" # Launch the job on boot.
gosu launch "..." --launch="on:event" # Launch the job when an event is signalled via `gosu signal event`.
```

List all running applications:

```bash
gosu ls
gosu view "app-.*" # Real-time view of all applications matching the regex.
```

Stop an application:

```bash
gosu stop app_name
```

## Configuration

(Documentation TBD)

```js
// config.js
export default {
	ts: {
		exec: "./myserver.ts",
		n: 4,
		proxy: {
			host: "localhost:3000",
			listen: ":3000",
			sticky: true,
			method: "conn",
			retry_max: 5,
			retry_backoff: 100,
		},
	},
};
```

## Contributing

Contributions to gosu are welcome! Please read our [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on how to contribute.

## License

gosu is licensed under the BSD 3-Clause License. See [LICENSE](LICENSE) for the full license text.
