CREATE TABLE "todos" (
    "id" bigserial PRIMARY KEY,
    "title" varchar NOT NULL,
    "completed" boolean NOT NULL DEFAULT false,
    "created_at" timestamptz NOT NULL DEFAULT (now())
);
