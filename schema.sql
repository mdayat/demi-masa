CREATE TYPE account_type AS ENUM ('free', 'premium');
CREATE TYPE payment_status AS ENUM ('unpaid', 'success', 'failed');

CREATE TABLE "user" (
  id VARCHAR(255),
  name VARCHAR(255) NOT NULL,
  email VARCHAR(255) UNIQUE NOT NULL,
  phone_number VARCHAR(255) UNIQUE,
  phone_verified BOOLEAN DEFAULT FALSE NOT NULL,
  account_type account_type DEFAULT 'free' NOT NULL,
  upgraded_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
  deleted_at TIMESTAMPTZ,

  PRIMARY KEY (id)
);

CREATE TABLE "order" (
  id UUID DEFAULT gen_random_uuid(),
  user_id VARCHAR(255) NOT NULL,
  transaction_id VARCHAR(255) NOT NULL,
  amount INT NOT NULL,
  payment_method VARCHAR(255) NOT NULL,
  payment_status payment_status DEFAULT 'unpaid' NOT NULL,

  PRIMARY KEY (id),
  CONSTRAINT fk_user
    FOREIGN KEY (user_id)
    REFERENCES "user" (id)
    ON UPDATE CASCADE
    ON DELETE CASCADE
);