using Go = import "../../../../../zombiezen.com/go/capnproto2/go.capnp";
@0xebd3d6337e485727;
$Go.package("capnproto");
$Go.import("sourcegraph.com/sourcegraph/appdash/opentracing/capnproto");

struct TracerState {
  traceid @0 :UInt64;
  spanid @1 :UInt64;
  sampled @2 :Bool;
}

struct Baggage {
  items @0 :List(Item);
  struct Item {
    key @0 :Text;
    val @1 :Text;
  }
}
