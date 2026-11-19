# btree

[![GoDoc](https://pkg.go.dev/badge/github.com/ajwerner/btree)](https://pkg.go.dev/github.com/ajwerner/btree)
![Beta](https://img.shields.io/badge/status-beta-yellow)

A Go generic library providing copy-on-write B-tree data structures including maps, sets, interval trees, and order-statistic trees. All variants share a common augmented B-tree implementation and support O(1) lazy cloning.

**Note:** This library is still in beta. Please report any issues on the [GitHub issue tracker](https://github.com/ajwerner/btree/issues).

Read more about the design in the [blog post](./blog/blog.md).

## Interval Trees

The `interval` package provides interval trees for efficiently finding all intervals that overlap a query range. The iterator supports `FirstOverlap()` and `NextOverlap()` methods for querying overlapping intervals.

## Order-Statistic Trees

The `orderstat` package provides order-statistic trees that support O(log n) rank queries and nth element selection. The iterator adds `Rank()` and `SeekNth()` methods for efficient positional queries.

## License

Copyright 2021 Andrew Werner. Licensed under the Apache License, Version 2.0.
