# Comment Safety Rules

- **CRITICAL RESTRICTION**: Never, ever post or submit comments to the live production website at `https://endgameviable.com` (or via its live API endpoint `https://api.endgameviable.com` / table `endgameviable_comments`) for testing or verification purposes.
- All comments validation and integration testing must be performed locally using mock data, local unit tests, or LocalStack (using `http://localhost:4566` and `spiritriot_comments`).
