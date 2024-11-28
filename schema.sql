CREATE TYPE account_type AS ENUM ('free', 'premium');
CREATE TYPE payment_status AS ENUM ('unpaid', 'paid', 'success', 'failed');

CREATE TABLE "user" (
  id VARCHAR(255),
  name VARCHAR(255) NOT NULL,
  email VARCHAR(255) UNIQUE NOT NULL,
  phone_number VARCHAR(255) UNIQUE,
  phone_verified BOOLEAN DEFAULT FALSE NOT NULL,
  account_type account_type DEFAULT 'free' NOT NULL,
  upgraded_at TIMESTAMPTZ,
  expired_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,

  PRIMARY KEY (id)
);

CREATE TABLE coupon (
  code VARCHAR(255),
  influencer_username VARCHAR(255) NOT NULL,
  quota SMALLINT NOT NULL,
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
  deleted_at TIMESTAMPTZ,
  
  PRIMARY KEY (code)
);

CREATE TABLE "order" (
  id UUID,
  user_id VARCHAR(255) NOT NULL,
  transaction_id VARCHAR(255) NOT NULL,
  coupon_code VARCHAR(255),
  amount INT NOT NULL,
  subscription_duration INT NOT NULL,
  payment_method VARCHAR(255) NOT NULL,
  payment_url VARCHAR(255) NOT NULL,
  payment_status payment_status DEFAULT 'unpaid' NOT NULL,
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
  paid_at TIMESTAMPTZ,
  expired_at TIMESTAMPTZ,

  PRIMARY KEY (id),

  CONSTRAINT fk_user
    FOREIGN KEY (user_id)
    REFERENCES "user" (id)
    ON UPDATE CASCADE
    ON DELETE CASCADE,

  CONSTRAINT fk_coupon
    FOREIGN KEY (coupon_code)
    REFERENCES coupon (code)
    ON UPDATE CASCADE
    ON DELETE CASCADE
);
