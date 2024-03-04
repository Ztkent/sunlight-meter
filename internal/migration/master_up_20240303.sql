CREATE TABLE IF NOT EXISTS "sunlight" (
    "id" SERIAL PRIMARY KEY,
    "job_id" varchar(255) NOT NULL,
    "lux" varchar(255) NOT NULL,
    "full_spectrum" varchar(255) NOT NULL,
    "visible" varchar(255) NOT NULL,
    "infrared" varchar(255) NOT NULL,
    "time" timestamp DEFAULT CURRENT_TIMESTAMP,
);