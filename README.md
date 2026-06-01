# Re-Implementation of xyrun in Go

This is a re-implementation of [xyrun](https://github.com/pixlcore/xyrun) in Go.
The idea is to get rid of NodeJS as dependency and provide a single binary that can be used as a drop-in replacement for the original xyrun.
This should help reduce the size of your Docker images that include xyrun, as you don't need to install NodeJS anymore.

I'll try to integrate changes from the upstream repository into this one asap.

## Usage

Read the upstream repository for usage instructions: https://github.com/pixlcore/xyrun

## License

See [LICENSE](LICENSE) in this repository.
