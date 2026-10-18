# Sourced, not run: the one place that names production's database. It keeps the environment's name
# (critical_prod) although the app was renamed (ADR 0024 step 8.5); scripts/fly-db-provision.sh derives
# the same name from the environment (critical_<env>).
prod_db="critical_prod"
