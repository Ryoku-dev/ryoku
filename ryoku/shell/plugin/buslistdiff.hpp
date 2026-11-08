#pragma once

#include <QList>
#include <QString>
#include <QStringList>

// keys must be unique, rows on the longest in-order run never move so a reorder stays cheap
namespace buslistdiff {

struct Op {
    enum Kind { Remove, Insert, Move };
    Kind kind;
    // Move uses QList::move semantics, to is the row's index afterwards
    int row = 0;
    int count = 1;
    int to = 0;
    int source = 0;
};

QList<Op> diff(const QStringList &before, const QStringList &after);

QStringList replay(QStringList before, const QStringList &after, const QList<Op> &ops);

}
