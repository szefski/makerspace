
DROP SCHEMA IF EXISTS makerspace CASCADE;
CREATE SCHEMA makerspace;
ALTER DATABASE makerspace SET search_path TO makerspace, pg_catalog;

CREATE TABLE member (
	id serial PRIMARY KEY,
	username text NOT NULL UNIQUE,
	name text NOT NULL,
	password_key character(64),
	password_salt character(64) UNIQUE,
	-- NULL indicates unverified e-mail
	-- TODO: e-mail uniqueness requires case-insensitive check
	email text UNIQUE,
	key_card character(8) UNIQUE,
	avatar_tmpl text,
	telephone text,
	agreed_to_terms boolean NOT NULL DEFAULT false,
	registered timestamp(0) with time zone NOT NULL DEFAULT now(),
	stripe_customer_id text
);
CREATE TABLE email_verification_token (
	member integer PRIMARY KEY REFERENCES member,
	email text NOT NULL,
	token character(64) NOT NULL,
	time timestamp(0) with time zone NOT NULL DEFAULT now()
);
CREATE TABLE reset_password_token (
	member integer PRIMARY KEY REFERENCES member,
	token character(64) NOT NULL,
	time timestamp(0) with time zone NOT NULL DEFAULT now()
);
CREATE TYPE admin_privilege AS ENUM (
	'approve-member',
	'modify-member',
	'revoke-member',
	'do-transactions');
CREATE TABLE administrator (
	member integer PRIMARY KEY REFERENCES member,
	privileges admin_privilege[]
);
CREATE TABLE student (
	member integer PRIMARY KEY REFERENCES member,
	institution text,
	student_email text,
	graduation_date date
);
CREATE TABLE session_http (
	token character(64) PRIMARY KEY,
	member integer NOT NULL REFERENCES member,
	sign_in_time timestamp(0) with time zone NOT NULL DEFAULT now(),
	last_seen timestamp(0) with time zone NOT NULL DEFAULT now(),
	expires timestamp(0) with time zone
);
CREATE TABLE pending_subscription (
	member integer NOT NULL REFERENCES member,
	requested_at timestamp(0) with time zone NOT NULL DEFAULT now(),
	plan_id text NOT NULL,
	UNIQUE (member, plan_id)
);
CREATE TABLE membership_cancellation (
	member integer NOT NULL REFERENCES member,
	time timestamp(0) with time zone NOT NULL DEFAULT now(),
	reason text
);
CREATE TABLE sent_emails (
	time timestamp(0) with time zone NOT NULL DEFAULT now(),
	from_address text,
	to_address text[],
	body text
);
CREATE TABLE storage (
	number integer NOT NULL,
	plan_id text NOT NULL,
	available boolean NOT NULL DEFAULT true,
	size integer,
	PRIMARY KEY (number, plan_id)
);
CREATE TABLE storage_waitlist (
	time timestamp(0) with time zone NOT NULL DEFAULT now(),
	member integer NOT NULL REFERENCES member,
	plan_id text NOT NULL,
	-- NULL signifies waiting for any number
	number integer,
	FOREIGN KEY (number, plan_id) REFERENCES storage
);
