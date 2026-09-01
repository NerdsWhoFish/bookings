# Contributing

Issues and pull requests are welcome. Keep the project narrow: Google Calendar scheduling, meeting types, theming, and the request-driven deployment model.

Before opening a pull request:

```console
make test
make lint
docker build -t bookings:local .
```

Frontend changes should cover keyboard use, reduced motion, failure states, and the supported small-screen range. Backend changes around slot claims or Google event creation need a concurrency or retry test.

New infrastructure should preserve `image_digest = null` bootstrap mode and zero minimum Cloud Run instances. A worker, scheduler, reminder system, payment flow, or arbitrary theme injection needs an architecture decision before code.

By contributing, you agree that your contribution is licensed under the MIT License.
