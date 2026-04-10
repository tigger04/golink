A simple link forwarder. We only ever return HTTP redirect codes, nothing else. Say this project has the domain go.tigger.dev
- go.tigger.dev/amazon/[ASIN] - redirect to the amazon product page in the appropriate local market (amazon.ie, amazon.co.uk, amazon.com, etc) based on IP geolocation
- may be expanded for other uses, so keep the architecture as simple and modular as possible. Expect to support non-retail links and uses in the future, so design with extensibility in mind.
