# Quorum controller

This controller manages a group of instances that form a quorum.  Instances in
a quorum are treated specially (and differently from those managed by a scaler)
since the number of instances must be managed more strictly.

Instances within a quorum are also different from scaler-managed instances
as they are heterogeneous.  Instances are identified by static IP addresses,
which allows them to self-identify without the need for additional external
coordination.
