#!/usr/bin/env bash
#
# tag.sh - 为根模块和所有子模块创建版本 tag
#
# 用法:
#   ./scripts/tag.sh <version>           # 创建 tag（本地）
#   ./scripts/tag.sh <version> --push    # 创建并推送到远程
#   ./scripts/tag.sh <version> --dry-run # 预览将创建的 tag
#
# 示例:
#   ./scripts/tag.sh v0.2.0
#   ./scripts/tag.sh v0.2.0 --push
#   ./scripts/tag.sh v0.2.0 --dry-run
#

set -euo pipefail

# ── 颜色输出 ───────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

info()  { echo -e "${CYAN}>>>${NC} $*"; }
ok()    { echo -e "${GREEN} ✓${NC} $*"; }
warn()  { echo -e "${YELLOW} !${NC} $*"; }
err()   { echo -e "${RED} ✗${NC} $*"; }

# ── 参数校验 ───────────────────────────────────────────────
if [[ $# -lt 1 ]]; then
    err "用法: $0 <version> [--push|--dry-run]"
    echo ""
    echo "示例:"
    echo "  $0 v0.2.0              # 仅创建本地 tag"
    echo "  $0 v0.2.0 --push       # 创建并推送到远程"
    echo "  $0 v0.2.0 --dry-run    # 预览将创建的 tag"
    exit 1
fi

VERSION="$1"
ACTION="${2:-}"

if [[ "$ACTION" != "--push" && "$ACTION" != "--dry-run" && "$ACTION" != "" ]]; then
    err "未知选项: $ACTION"
    err "有效选项: --push, --dry-run"
    exit 1
fi

# 校验版本号格式 (vX.Y.Z 或 vX.Y.Z-pre.N 等)
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$ ]]; then
    err "版本号格式无效: $VERSION"
    err "期望格式: vX.Y.Z (如 v0.2.0, v1.0.0-rc.1)"
    exit 1
fi

# ── 自动发现子模块 ───────────────────────────────────────
# 从文件系统中扫描所有包含 go.mod 的子目录
mapfile -t SUB_MODULES < <(
    find . -name go.mod -not -path './.git/*' -not -path './go.mod' | \
        sed 's|./\(.*\)/go.mod|\1|' | \
        sort
)

if [[ ${#SUB_MODULES[@]} -eq 0 ]]; then
    warn "未发现子模块"
fi

# ── 检查 git 状态 ────────────────────────────────────────
if ! git rev-parse --is-inside-work-tree &>/dev/null; then
    err "不在 git 仓库中"
    exit 1
fi

COMMIT=$(git rev-parse HEAD)
BRANCH=$(git rev-parse --abbrev-ref HEAD)

info "仓库: ${BOLD}$(git remote get-url origin 2>/dev/null || echo 'unknown')${NC}"
info "分支: ${BOLD}$BRANCH${NC}"
info "提交: ${BOLD}${COMMIT:0:8}${NC}"
info "版本: ${BOLD}$VERSION${NC}"
echo ""

# ── 检查工作区是否干净 ─────────────────────────────────
if [[ -n "$(git status --porcelain 2>/dev/null)" ]]; then
    warn "工作区有未提交的更改，建议先提交"
    echo ""
fi

# ── 检查 tag 是否已存在 ─────────────────────────────────
EXISTING_TAGS=()
check_tags=("$VERSION")
for sub in "${SUB_MODULES[@]}"; do
    check_tags+=("$sub/$VERSION")
done

for tag in "${check_tags[@]}"; do
    if git rev-parse "$tag" &>/dev/null; then
        EXISTING_TAGS+=("$tag")
    fi
done

if [[ ${#EXISTING_TAGS[@]} -gt 0 ]]; then
    err "以下 tag 已存在:"
    for tag in "${EXISTING_TAGS[@]}"; do
        err "  - $tag"
    done
    echo ""
    err "请使用其他版本号，或先删除已有 tag:"
    echo "  git tag -d <tag>              # 删除本地 tag"
    echo "  git push origin :refs/tags/<tag>  # 删除远程 tag"
    exit 1
fi

# ── 生成 tag 列表 ──────────────────────────────────────
ALL_TAGS=("$VERSION")
for sub in "${SUB_MODULES[@]}"; do
    ALL_TAGS+=("$sub/$VERSION")
done

# ── Dry-run 模式 ──────────────────────────────────────
if [[ "$ACTION" == "--dry-run" ]]; then
    info "${BOLD}预览将创建的 tag:${NC}"
    echo ""
    for tag in "${ALL_TAGS[@]}"; do
        echo -e "  ${GREEN}+${NC} $tag  →  ${COMMIT:0:8}"
    done
    echo ""
    info "共 ${#ALL_TAGS[@]} 个 tag (1 根模块 + ${#SUB_MODULES[@]} 子模块)"
    echo ""
    echo "执行以下命令创建:"
    echo "  ./scripts/tag.sh $VERSION"
    echo ""
    echo "创建并推送:"
    echo "  ./scripts/tag.sh $VERSION --push"
    exit 0
fi

# ── 创建 tag ──────────────────────────────────────────
info "${BOLD}创建 tag...${NC}"
echo ""

FAILED=()

for tag in "${ALL_TAGS[@]}"; do
    if git tag "$tag" "$COMMIT" 2>/dev/null; then
        ok "$tag"
    else
        err "创建失败: $tag"
        FAILED+=("$tag")
    fi
done

echo ""

if [[ ${#FAILED[@]} -gt 0 ]]; then
    err "部分 tag 创建失败，请检查后再试"
    exit 1
fi

info "成功创建 ${#ALL_TAGS[@]} 个 tag (1 根模块 + ${#SUB_MODULES[@]} 子模块)"
echo ""

# ── 推送 tag ──────────────────────────────────────────
if [[ "$ACTION" == "--push" ]]; then
    info "${BOLD}推送到远程...${NC}"
    echo ""
    if git push origin --tags 2>&1; then
        echo ""
        ok "所有 tag 已推送到远程 ✓"
    else
        echo ""
        err "推送失败，请检查网络连接和仓库权限"
        err "tag 已在本地创建，可稍后手动推送:"
        echo "  git push origin --tags"
        exit 1
    fi
else
    echo "Tag 已在本地创建，未推送到远程。推送方法:"
    echo ""
    echo "  git push origin --tags"
    echo ""
    echo "或只推送特定子模块 tag:"
    echo ""
    for tag in "${ALL_TAGS[@]}"; do
        echo "  git push origin $tag"
    done
fi

echo ""
ok "完成!"
