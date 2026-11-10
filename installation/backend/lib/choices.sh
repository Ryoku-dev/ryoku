#!/usr/bin/env bash

# The Go installers embed this manifest. The ISO backends read the same file from
# the staged repository so choice order, defaults and package drops cannot drift.

declare -gA RYOKU_CHOICE_PACKAGES=()
declare -gA RYOKU_CHOICE_TARGET=()
declare -gA RYOKU_CHOICE_REASON_BY_PACKAGE=()
declare -gA RYOKU_CHOICE_POLICY=()
declare -gA RYOKU_CHOICE_AVAILABLE=()
declare -gA RYOKU_CHOICE_UNAVAILABLE_REASON=()
declare -gA RYOKU_CHOICE_SEEN=()
declare -ga RYOKU_BROWSER_CHOICES=()
declare -ga RYOKU_LOGIN_SHELL_CHOICES=()
declare -ga RYOKU_FILE_MANAGER_CHOICES=()
declare -ga RYOKU_COMPOSITOR_CHOICES=()
declare -g RYOKU_CHOICE_EDITION=

_ryoku_choice_manifest() {
  local here
  here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
  for path in \
    "${RYOKU_INSTALLER_CHOICES_FILE:-}" \
    "${RYOKU_REPO:-/usr/share/ryoku}/installation/installerplan/choices.tsv" \
    "$here/../../installerplan/choices.tsv"; do
    [[ -n $path && -f $path ]] && { printf '%s\n' "$path"; return 0; }
  done
  return 1
}

_ryoku_choice_ids() {
  case $1 in
    browser) printf '%s\n' "${RYOKU_BROWSER_CHOICES[@]}" ;;
    login-shell) printf '%s\n' "${RYOKU_LOGIN_SHELL_CHOICES[@]}" ;;
    file-manager) printf '%s\n' "${RYOKU_FILE_MANAGER_CHOICES[@]}" ;;
    compositor) printf '%s\n' "${RYOKU_COMPOSITOR_CHOICES[@]}" ;;
  esac
}

_ryoku_choice_allowed() {
  local kind=$1 joined='' id
  while IFS= read -r id; do
    [[ -n $id ]] || continue
    joined+="${joined:+, }$id"
  done < <(_ryoku_choice_ids "$kind")
  printf '%s' "$joined"
}

ryoku_choice_init() {
  local edition=$1 table=${2:-} manifest record kind id packages fresh existing
  local arch target notes pkg key
  local -a choice_packages
  manifest=$(_ryoku_choice_manifest) || die 'installer choice manifest is missing'

  RYOKU_BROWSER_CHOICES=()
  RYOKU_LOGIN_SHELL_CHOICES=()
  RYOKU_COMPOSITOR_CHOICES=()
  RYOKU_FILE_MANAGER_CHOICES=()
  RYOKU_CHOICE_EDITION=$edition

  while IFS=$'\t' read -r record kind id _ packages _ fresh existing _; do
    [[ -n $record && $record != \#* ]] || continue
    [[ $record != record ]] || continue
    case $record in
      choice)
        key="$kind:$id"
        # A dash is the manifest's empty cell. Kept as one here it would split
        # into a package literally named "-": the choice would read as
        # unavailable on every edition, and dropping the ones not picked would
        # try to remove it. The Go side treats the column the same way.
        [[ $packages == - ]] && packages=
        RYOKU_CHOICE_PACKAGES[$key]=$packages
        RYOKU_CHOICE_SEEN[$key]=1
        case $kind in
          browser) RYOKU_BROWSER_CHOICES+=("$id") ;;
          login-shell) RYOKU_LOGIN_SHELL_CHOICES+=("$id") ;;
          file-manager) RYOKU_FILE_MANAGER_CHOICES+=("$id") ;;
          compositor) RYOKU_COMPOSITOR_CHOICES+=("$id") ;;
          *) die "installer choice manifest has unknown kind '%s'" "$kind" ;;
        esac
        ;;
      policy)
        if [[ $id == fresh ]]; then
          RYOKU_CHOICE_POLICY["$kind:fresh"]=$fresh
        else
          RYOKU_CHOICE_POLICY["$kind:existing"]=$existing
        fi
        ;;
      *) die "installer choice manifest has unknown record '%s'" "$record" ;;
    esac
  done < "$manifest"

  if [[ -n $table ]]; then
    [[ -f $table ]] || die "%s edition package table is missing: %s" "$edition" "$table"
    while IFS=$'\t' read -r arch target _ notes; do
      [[ -n $arch && $arch != \#* ]] || continue
      if [[ $arch == arch ]]; then
        RYOKU_CHOICE_EDITION=${target^}
        continue
      fi
      RYOKU_CHOICE_TARGET[$arch]=$target
      RYOKU_CHOICE_REASON_BY_PACKAGE[$arch]=$notes
    done < "$table"
  fi

  for kind in browser login-shell file-manager compositor; do
    while IFS= read -r id; do
      [[ -n $id ]] || continue
      key="$kind:$id"
      RYOKU_CHOICE_AVAILABLE[$key]=1
      RYOKU_CHOICE_UNAVAILABLE_REASON[$key]=
      [[ -n $table ]] || continue
      IFS=, read -ra choice_packages <<< "${RYOKU_CHOICE_PACKAGES[$key]}"
      for pkg in "${choice_packages[@]}"; do
        target=${RYOKU_CHOICE_TARGET[$pkg]-}
        if [[ -n $target && $target != - ]]; then
          continue
        fi
        RYOKU_CHOICE_AVAILABLE[$key]=0
        RYOKU_CHOICE_UNAVAILABLE_REASON[$key]=${RYOKU_CHOICE_REASON_BY_PACKAGE[$pkg]-}
        [[ -n ${RYOKU_CHOICE_UNAVAILABLE_REASON[$key]} ]] \
          || RYOKU_CHOICE_UNAVAILABLE_REASON[$key]="$pkg is not listed in the $RYOKU_CHOICE_EDITION edition table."
        break
      done
    done < <(_ryoku_choice_ids "$kind")
  done
}

ryoku_choice_validate() {
  local kind=$1 id=$2 key="$1:$2" allowed label
  allowed=$(_ryoku_choice_allowed "$kind")
  if [[ -z ${RYOKU_CHOICE_SEEN[$key]-} ]]; then
    case $kind in
      browser) die "RYOKU_BROWSER='%s' is not one of the shipped browsers (%s)" "$id" "$allowed" ;;
      login-shell) die "RYOKU_LOGIN_SHELL='%s' is not one of the shipped login shells (%s)" "$id" "$allowed" ;;
      file-manager) die "RYOKU_FILE_MANAGER='%s' is not one of the shipped file managers (%s)" "$id" "$allowed" ;;
      compositor) die "RYOKU_COMPOSITOR='%s' is not one of the shipped window managers (%s)" "$id" "$allowed" ;;
    esac
  fi
  if [[ ${RYOKU_CHOICE_AVAILABLE[$key]-0} != 1 ]]; then
    case $kind in
      browser) label=Browser ;;
      login-shell) label='Login shell' ;;
      file-manager) label='File manager' ;;
      compositor) label='Window manager' ;;
    esac
    die "%s choice '%s' is unavailable on %s: %s" "$label" "$id" "$RYOKU_CHOICE_EDITION" "${RYOKU_CHOICE_UNAVAILABLE_REASON[$key]}"
  fi
}

_ryoku_append_drop() {
  local package=$1
  [[ ",${RYOKU_DROP_PACKAGES:-}," == *,"$package",* ]] \
    || RYOKU_DROP_PACKAGES="${RYOKU_DROP_PACKAGES:+$RYOKU_DROP_PACKAGES,}$package"
}

_ryoku_remove_drop() {
  local remove=$1 package next=
  local -a packages=()
  IFS=, read -ra packages <<<"${RYOKU_DROP_PACKAGES:-}"
  for package in "${packages[@]}"; do
    [[ -n $package && $package != "$remove" ]] || continue
    next="${next:+$next,}$package"
  done
  RYOKU_DROP_PACKAGES=$next
}

ryoku_choice_validate_env() {
  local id
  case ${RYOKU_RASHIN:-0} in
    0|1) ;;
    *) die "RYOKU_RASHIN must be 0 or 1 (got '%s')" "${RYOKU_RASHIN}" ;;
  esac
  case ${RYOKU_CACHYOS_LAYER:-0} in
    0|1) ;;
    *) die "RYOKU_CACHYOS_LAYER must be 0 or 1 (got '%s')" "${RYOKU_CACHYOS_LAYER}" ;;
  esac
  [[ -n ${RYOKU_HIDE_APPS:-} ]] || return 0
  [[ $RYOKU_HIDE_APPS != ,* && $RYOKU_HIDE_APPS != *, && $RYOKU_HIDE_APPS != *,,* ]] \
    || die "RYOKU_HIDE_APPS must be a comma list of plain *.desktop file names"
  IFS=, read -ra ids <<<"$RYOKU_HIDE_APPS"
  for id in "${ids[@]}"; do
    [[ $id =~ ^[A-Za-z0-9][A-Za-z0-9._+-]*\.desktop$ ]] \
      || die "RYOKU_HIDE_APPS contains invalid desktop id '%s' (use plain *.desktop file names)" "$id"
  done
}

ryoku_choice_apply_fresh() {
  local edition=$1 table=${2:-} kind id selected package key
  local -a packages
  ryoku_choice_init "$edition" "$table"
  : "${RYOKU_BROWSER:=${RYOKU_CHOICE_POLICY[browser:fresh]}}"
  : "${RYOKU_LOGIN_SHELL:=${RYOKU_CHOICE_POLICY[login-shell:fresh]}}"
  : "${RYOKU_FILE_MANAGER:=${RYOKU_CHOICE_POLICY[file-manager:fresh]}}"
  : "${RYOKU_RASHIN:=0}"
  : "${RYOKU_HIDE_APPS:=}"
  : "${RYOKU_CACHYOS_LAYER:=0}"
  ryoku_choice_validate browser "$RYOKU_BROWSER"
  ryoku_choice_validate login-shell "$RYOKU_LOGIN_SHELL"
  ryoku_choice_validate file-manager "$RYOKU_FILE_MANAGER"
  ryoku_choice_validate_env
  if [[ -n ${RYOKU_COMPOSITOR:-} ]]; then
    ryoku_choice_validate compositor "$RYOKU_COMPOSITOR"
  fi
  for kind in browser login-shell file-manager; do
    case $kind in
      browser) selected=$RYOKU_BROWSER ;;
      login-shell) selected=$RYOKU_LOGIN_SHELL ;;
      file-manager) selected=$RYOKU_FILE_MANAGER ;;
    esac
    while IFS= read -r id; do
      [[ -n $id && $id != "$selected" ]] || continue
      key="$kind:$id"
      IFS=, read -ra packages <<< "${RYOKU_CHOICE_PACKAGES[$key]}"
      for package in "${packages[@]}"; do _ryoku_append_drop "$package"; done
    done < <(_ryoku_choice_ids "$kind")
  done
  if [[ $RYOKU_RASHIN == 0 ]]; then
    _ryoku_append_drop ryoku-rashin
  else
    _ryoku_remove_drop ryoku-rashin
  fi
  export RYOKU_BROWSER RYOKU_LOGIN_SHELL RYOKU_FILE_MANAGER
  export RYOKU_RASHIN RYOKU_HIDE_APPS RYOKU_CACHYOS_LAYER RYOKU_DROP_PACKAGES
}
