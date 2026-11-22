# Dated-flight model

Issue #28 adds an internal Go contract. A future adapter supplies candidate commercial flight legs through `flight.Source`. Existing Sky commands operate on observed aircraft and callsigns. Fleet membership is a separate concept.

- A lookup uses a full IATA or ICAO flight designator and a service date in the departure timezone. An optional departure airport distinguishes legs. Adapters must normalize airport codes consistently for origin filtering.
- Operating and marketing designators can both match the same leg. Adapters should consolidate duplicate representations of that leg before returning candidates. Multiple matching legs return an ambiguity error.
- An explicit provider service date takes precedence because a flight can depart after midnight or be rescheduled. When absent, the resolver derives the date from the scheduled departure gate instant and its IANA timezone. Adapters must resolve timezone-less or DST-ambiguous wall times before supplying an instant. A matching candidate without enough date or origin information prevents a definitive match.
- Each leg carries provider identity, retrieval time and an optional provider reference. Aircraft registration is nullable. Gate and runway events each represent scheduled, estimated and actual instants independently. A chronology check flags reversed actual events while preserving the original values.
- Normalized status and the provider's original status are separate. An unrecognized status maps to unknown. A provider can report an unsupported lookup separately from a successful lookup with no matches.

The synthetic tests exercise date reuse, codeshares, multiple legs, missing information, cancellation, midnight and DST boundaries, and conflicting timestamps. Commercial adoption remains subject to the [provider evaluation](PROVIDER_EVALUATION.md).
