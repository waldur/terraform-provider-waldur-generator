# GitHub Workflow Setup Guide

This repository contains a GitHub Action workflow (`.github/workflows/deploy.yml`) that automatically generates the Terraform provider code and pushes it to the `waldur/terraform-provider-waldur` repository.

To enable this workflow, you must configure a **GitHub Personal Access Token (PAT)**.

## Why is a PAT required?

Even if both repositories are in the same organization, the default `GITHUB_TOKEN` used by Actions only has permissions for the repository where the workflow runs (`terraform-provider-waldur-generator`). It cannot push code to other repositories (`terraform-provider-waldur`).

## Step-by-Step Configuration

### 1. Generate a Fine-grained Personal Access Token

1. Log in to GitHub and go to **Settings** (click your profile picture > Settings).
2. In the left sidebar, scroll down to **Developer settings**.
3. Click on **Personal access tokens** -> **Fine-grained tokens**.
4. Click **Generate new token**.
5. **Name**: Give it a clear name, e.g., `Waldur Provider Generator Push`.
6. **Expiration**: Choose an expiration period (standard is 30-90 days, or "No expiration" if your org policy allows and you want fully automated maintenance, though rotation is recommended).
7. **Resource owner**: Select the organization `waldur` (if prompted).
8. **Repository access**:
    * Select **Only select repositories**.
    * Select the target repository: `waldur/terraform-provider-waldur`.
9. **Permissions**:
    * Click on **Repository permissions**.
    * Find **Contents** and change it to **Read and Write**.
    * (Optional) If correct metadata is needed, you might also want **Metadata** (usually Read-only by default).
10. Click **Generate token**.
11. **Copy the token immediately**. You won't be able to see it again.

### 2. Add the Secret to the Generator Repository

1. Navigate to the **generator repository**: `waldur/terraform-provider-waldur-generator`.
2. Go to **Settings** (top tab).
3. In the left sidebar, go to **Secrets and variables** -> **Actions**.
4. Under "Repository secrets", click **New repository secret**.
5. **Name**: `WALDUR_PROVIDER_PUSH_TOKEN` (This must match the name in `deploy.yml`).
6. **Secret**: Paste the PAT you copied in step 1.
7. Click **Add secret**.

## Verification

To verify the setup:

1. Go to the **Actions** tab in the `terraform-provider-waldur-generator` repository.
2. Select the **Generate and Push Provider** workflow.
3. Click **Run workflow** -> **Run workflow** (on `main` branch).
4. If successful, the workflow should complete green, and you should see a new commit in `waldur/terraform-provider-waldur`.
