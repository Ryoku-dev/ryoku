#pragma once

#include <QList>
#include <QString>
#include <QStringList>

// The row moves that turn one keyed list into another: removals first (from
// the bottom, so earlier indices stay valid), then a walk down the new order
// that inserts new keys and moves the rest into place. Rows on the longest run
// that already keeps its relative order never move, so a reorder costs one move
// per row that left that run. Keys must be unique in each list.
namespace buslistdiff {

struct Op {
    enum Kind { Remove, Insert, Move };
    Kind kind;
    // Remove: rows [row, row + count). Insert: count new rows at row, taken
    // from the new list starting at source. Move: the row at row goes to
    // index to (QList::move semantics: to is its index afterwards).
    int row = 0;
    int count = 1;
    int to = 0;
    int source = 0;
};

QList<Op> diff(const QStringList &before, const QStringList &after);

// Applies ops to a copy of before, for tests and for checking a diff.
QStringList replay(QStringList before, const QStringList &after, const QList<Op> &ops);

}
