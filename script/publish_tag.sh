#!/bin/bash

# BingPaperDesktop Tag Publication Script
# This script automates the process of creating and pushing a new tag to trigger the release workflow.

set -e

# Change to the project root directory
cd "$(dirname "$0")/.."

# 1. Update local code
echo "Updating local code..."
git pull origin master

# 2. Check current branch
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$CURRENT_BRANCH" != "master" ]; then
    echo "Error: You must be on the 'master' branch to publish a tag."
    exit 1
fi

# 3. Check if local master is up-to-date with remote master
git fetch origin master
LOCAL=$(git rev-parse HEAD)
REMOTE=$(git rev-parse origin/master)

if [ "$LOCAL" != "$REMOTE" ]; then
    echo "Error: Local branch is not up-to-date with remote. Please push or pull changes first."
    exit 1
fi

# 4. Get tag name from user
if [ -z "$1" ]; then
    read -p "Enter tag name (e.g., v1.0.0): " TAG_NAME
else
    TAG_NAME=$1
fi

if [[ ! $TAG_NAME =~ ^v ]]; then
    echo "Error: Tag name must start with 'v' (e.g., v1.0.0)."
    exit 1
fi

# 5. Create and force push the tag
echo "Creating tag $TAG_NAME..."
git tag -f "$TAG_NAME"

echo "Pushing tag $TAG_NAME to remote..."
git push origin "$TAG_NAME" -f

echo "Successfully published tag $TAG_NAME. The release workflow should now be triggered."
