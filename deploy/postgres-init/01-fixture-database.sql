-- Run once by the postgres image when the compose volume is created (docker-entrypoint-initdb.d).
-- `just dev` keeps two databases apart: `critical` with the real catalogue, `critical_fixture` with
-- the fixture catalogue and seeds of api/testdata. The image creates the first (POSTGRES_DB).
CREATE DATABASE critical_fixture;
