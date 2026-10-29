--
-- PostgreSQL database dump
--

\restrict 9GnhRrLzS2qMK4XQomifpIcRweLmEzOeaO7Q0sMZYqZ2NIhOrIvvBMsYZbjKSc7

-- Dumped from database version 18.6 (653b019)
-- Dumped by pg_dump version 18.6 (Ubuntu 18.6-1.pgdg24.04+2)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: public; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA public;


--
-- Name: SCHEMA public; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON SCHEMA public IS 'standard public schema';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: cart_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cart_items (
    id integer NOT NULL,
    user_id integer NOT NULL,
    menu_item_id integer NOT NULL,
    quantity integer DEFAULT 1 NOT NULL,
    price_kobo_at_addition integer NOT NULL
);


--
-- Name: cart_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.cart_items_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: cart_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.cart_items_id_seq OWNED BY public.cart_items.id;


--
-- Name: favorites; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.favorites (
    user_id integer NOT NULL,
    menu_item_id integer NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: menu_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.menu_items (
    id integer NOT NULL,
    name text NOT NULL,
    description text,
    price_kobo integer NOT NULL,
    image_url text,
    category text DEFAULT ''::text NOT NULL,
    vendor_id integer DEFAULT 1
);


--
-- Name: menu_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.menu_items_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: menu_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.menu_items_id_seq OWNED BY public.menu_items.id;


--
-- Name: order_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.order_items (
    id integer NOT NULL,
    order_id integer NOT NULL,
    menu_item_id integer NOT NULL,
    quantity integer NOT NULL,
    price_kobo integer NOT NULL
);


--
-- Name: order_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.order_items_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: order_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.order_items_id_seq OWNED BY public.order_items.id;


--
-- Name: orders; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.orders (
    id integer NOT NULL,
    user_id integer NOT NULL,
    total_kobo integer NOT NULL,
    status character varying(50) DEFAULT 'pending'::character varying NOT NULL,
    payment_reference character varying(100),
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    vendor_id integer DEFAULT 1,
    delivery_method text DEFAULT 'pickup'::text NOT NULL,
    delivery_address text DEFAULT ''::text NOT NULL,
    delivery_phone text DEFAULT ''::text NOT NULL,
    delivery_fee_kobo integer DEFAULT 0 NOT NULL,
    delivery_state text DEFAULT ''::text NOT NULL,
    delivery_lga text DEFAULT ''::text NOT NULL,
    commission_kobo integer DEFAULT 0 NOT NULL,
    loyalty_points_redeemed integer DEFAULT 0 NOT NULL,
    CONSTRAINT orders_loyalty_points_redeemed_check CHECK (loyalty_points_redeemed >= 0),
    CONSTRAINT orders_delivery_fee_kobo_check CHECK ((delivery_fee_kobo >= 0)),
    CONSTRAINT orders_delivery_method_check CHECK ((delivery_method = ANY (ARRAY['pickup'::text, 'delivery'::text])))
);


--
-- Name: orders_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.orders_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: orders_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.orders_id_seq OWNED BY public.orders.id;


--
-- Name: password_reset_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.password_reset_tokens (
    token text NOT NULL,
    user_id integer NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    used boolean DEFAULT false NOT NULL
);


--
-- Name: vendor_payouts; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.vendor_payouts (
    id serial PRIMARY KEY,
    vendor_id integer NOT NULL,
    amount_kobo integer NOT NULL,
    note text DEFAULT ''::text NOT NULL,
    paid_at timestamp with time zone DEFAULT now() NOT NULL,
    recorded_by integer,
    CONSTRAINT vendor_payouts_amount_kobo_check CHECK ((amount_kobo > 0))
);


--
-- Name: phone_verifications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.phone_verifications (
    pin_id text NOT NULL,
    user_id integer NOT NULL,
    phone text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.sessions (
    token text NOT NULL,
    user_id integer NOT NULL,
    expires_at timestamp without time zone NOT NULL
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id integer NOT NULL,
    name text NOT NULL,
    first_name text NOT NULL DEFAULT ''::text,
    middle_name text NOT NULL DEFAULT ''::text,
    last_name text NOT NULL DEFAULT ''::text,
    email text NOT NULL,
    password text,
    auth_provider text DEFAULT 'local'::text NOT NULL,
    is_admin boolean DEFAULT false NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    profile_picture text,
    phone text,
    country text NOT NULL DEFAULT 'Nigeria'::text,
    state text NOT NULL DEFAULT ''::text,
    delivery_area text,
    terms_accepted_at timestamp with time zone,
    preferred_contact text NOT NULL DEFAULT 'email'::text,
    marketing_opt_in boolean DEFAULT false NOT NULL
);


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;


--
-- Name: vendor_applications; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.vendor_applications (
    id integer NOT NULL,
    user_id integer NOT NULL,
    business_name text NOT NULL,
    owner_name text NOT NULL,
    phone text NOT NULL,
    area text NOT NULL,
    address text NOT NULL,
    id_type text NOT NULL,
    online_link text DEFAULT ''::text NOT NULL,
    notes text DEFAULT ''::text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    admin_note text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    reviewed_at timestamp with time zone,
    review_method text DEFAULT ''::text NOT NULL,
    reviewed_by integer,
    CONSTRAINT vendor_applications_id_type_check CHECK ((id_type = ANY (ARRAY['nin'::text, 'drivers_licence'::text, 'voters_card'::text, 'passport'::text]))),
    CONSTRAINT vendor_applications_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'approved'::text, 'rejected'::text])))
);


--
-- Name: vendor_applications_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.vendor_applications_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: vendor_applications_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.vendor_applications_id_seq OWNED BY public.vendor_applications.id;


--
-- Name: vendor_payments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.vendor_payments (
    id integer NOT NULL,
    vendor_id integer NOT NULL,
    kind text NOT NULL,
    amount_kobo integer NOT NULL,
    reference text,
    status text DEFAULT 'pending'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    paid_at timestamp with time zone,
    CONSTRAINT vendor_payments_amount_kobo_check CHECK ((amount_kobo > 0)),
    CONSTRAINT vendor_payments_kind_check CHECK ((kind = ANY (ARRAY['setup'::text, 'renewal'::text]))),
    CONSTRAINT vendor_payments_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'paid'::text])))
);


--
-- Name: vendor_payments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.vendor_payments_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: vendor_payments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.vendor_payments_id_seq OWNED BY public.vendor_payments.id;


--
-- Name: vendors; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.vendors (
    id integer NOT NULL,
    name text NOT NULL,
    slug text NOT NULL,
    owner_user_id integer,
    status text DEFAULT 'pending'::text NOT NULL,
    setup_paid_at timestamp with time zone,
    paid_until timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    area text DEFAULT ''::text NOT NULL,
    address text DEFAULT ''::text NOT NULL,
    phone text DEFAULT ''::text NOT NULL,
    logo_url text DEFAULT ''::text NOT NULL,
    offers_pickup boolean DEFAULT true NOT NULL,
    offers_delivery boolean DEFAULT false NOT NULL,
    delivery_fee_kobo integer DEFAULT 0 NOT NULL,
    verified boolean DEFAULT false NOT NULL,
    payout_bank_name text DEFAULT ''::text NOT NULL,
    payout_bank_code text DEFAULT ''::text NOT NULL,
    payout_account_number text DEFAULT ''::text NOT NULL,
    payout_account_name text DEFAULT ''::text NOT NULL,
    payout_account_verified boolean DEFAULT false NOT NULL,
    payout_account_verified_at timestamp with time zone,
    CONSTRAINT vendors_delivery_fee_kobo_check CHECK ((delivery_fee_kobo >= 0)),
    CONSTRAINT vendors_status_check CHECK ((status = ANY (ARRAY['pending'::text, 'active'::text, 'suspended'::text])))
);


--
-- Name: vendor_delivery_areas; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.vendor_payout_verifications (
    token text NOT NULL,
    vendor_id integer NOT NULL,
    bank_name text NOT NULL,
    bank_code text NOT NULL,
    account_number text NOT NULL,
    account_name text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    used_at timestamp with time zone,
    CONSTRAINT vendor_payout_verifications_pkey PRIMARY KEY (token)
);

CREATE TABLE public.vendor_delivery_areas (
    id integer NOT NULL,
    vendor_id integer NOT NULL,
    state text NOT NULL,
    lga text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT vendor_delivery_areas_vendor_state_lga_key UNIQUE (vendor_id, state, lga)
);

CREATE SEQUENCE public.vendor_delivery_areas_id_seq START WITH 1 INCREMENT BY 1 CACHE 1;
ALTER SEQUENCE public.vendor_delivery_areas_id_seq OWNED BY public.vendor_delivery_areas.id;
ALTER TABLE ONLY public.vendor_delivery_areas ALTER COLUMN id SET DEFAULT nextval('public.vendor_delivery_areas_id_seq'::regclass);

--
-- Name: vendors_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.vendors_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: vendors_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.vendors_id_seq OWNED BY public.vendors.id;


--
-- Name: cart_items id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cart_items ALTER COLUMN id SET DEFAULT nextval('public.cart_items_id_seq'::regclass);


--
-- Name: menu_items id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.menu_items ALTER COLUMN id SET DEFAULT nextval('public.menu_items_id_seq'::regclass);


--
-- Name: order_items id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_items ALTER COLUMN id SET DEFAULT nextval('public.order_items_id_seq'::regclass);


--
-- Name: orders id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.orders ALTER COLUMN id SET DEFAULT nextval('public.orders_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);


--
-- Name: vendor_applications id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_applications ALTER COLUMN id SET DEFAULT nextval('public.vendor_applications_id_seq'::regclass);


--
-- Name: vendor_payments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_payments ALTER COLUMN id SET DEFAULT nextval('public.vendor_payments_id_seq'::regclass);


--
-- Name: vendors id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendors ALTER COLUMN id SET DEFAULT nextval('public.vendors_id_seq'::regclass);


--
-- Name: cart_items cart_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cart_items
    ADD CONSTRAINT cart_items_pkey PRIMARY KEY (id);


--
-- Name: cart_items cart_items_user_id_menu_item_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cart_items
    ADD CONSTRAINT cart_items_user_id_menu_item_id_key UNIQUE (user_id, menu_item_id);


--
-- Name: favorites favorites_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.favorites
    ADD CONSTRAINT favorites_pkey PRIMARY KEY (user_id, menu_item_id);


--
-- Name: menu_items menu_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.menu_items
    ADD CONSTRAINT menu_items_pkey PRIMARY KEY (id);


--
-- Name: order_items order_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_items
    ADD CONSTRAINT order_items_pkey PRIMARY KEY (id);


--
-- Name: orders orders_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_pkey PRIMARY KEY (id);


--
-- Name: password_reset_tokens password_reset_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_pkey PRIMARY KEY (token);


--
-- Name: sessions sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_pkey PRIMARY KEY (token);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_phone_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_phone_key UNIQUE (phone);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: vendor_applications vendor_applications_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_applications
    ADD CONSTRAINT vendor_applications_pkey PRIMARY KEY (id);


--
-- Name: vendor_payments vendor_payments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_payments
    ADD CONSTRAINT vendor_payments_pkey PRIMARY KEY (id);


--
-- Name: vendor_payments vendor_payments_reference_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_payments
    ADD CONSTRAINT vendor_payments_reference_key UNIQUE (reference);


--
-- Name: vendor_delivery_areas vendor_delivery_areas_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_delivery_areas
    ADD CONSTRAINT vendor_delivery_areas_pkey PRIMARY KEY (id);


-- Name: vendors vendors_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendors
    ADD CONSTRAINT vendors_pkey PRIMARY KEY (id);


-- Name: vendor_delivery_areas vendor_delivery_areas_vendor_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_delivery_areas
    ADD CONSTRAINT vendor_delivery_areas_vendor_id_fkey FOREIGN KEY (vendor_id) REFERENCES public.vendors(id) ON DELETE CASCADE;

CREATE INDEX idx_vendor_delivery_areas_vendor ON public.vendor_delivery_areas USING btree (vendor_id);


--
-- Name: vendors vendors_slug_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendors
    ADD CONSTRAINT vendors_slug_key UNIQUE (slug);


--
-- Name: idx_menu_items_vendor; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_menu_items_vendor ON public.menu_items USING btree (vendor_id);


--
-- Name: idx_orders_vendor; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_orders_vendor ON public.orders USING btree (vendor_id);


--
-- Name: idx_vendor_payments_vendor; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_vendor_payments_vendor ON public.vendor_payments USING btree (vendor_id);


--
-- Name: one_pending_application_per_user; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX one_pending_application_per_user ON public.vendor_applications USING btree (user_id) WHERE (status = 'pending'::text);


--
-- Name: cart_items cart_items_menu_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cart_items
    ADD CONSTRAINT cart_items_menu_item_id_fkey FOREIGN KEY (menu_item_id) REFERENCES public.menu_items(id) ON DELETE CASCADE;


--
-- Name: cart_items cart_items_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cart_items
    ADD CONSTRAINT cart_items_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: favorites favorites_menu_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.favorites
    ADD CONSTRAINT favorites_menu_item_id_fkey FOREIGN KEY (menu_item_id) REFERENCES public.menu_items(id) ON DELETE CASCADE;


--
-- Name: favorites favorites_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.favorites
    ADD CONSTRAINT favorites_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: menu_items menu_items_vendor_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.menu_items
    ADD CONSTRAINT menu_items_vendor_id_fkey FOREIGN KEY (vendor_id) REFERENCES public.vendors(id);


--
-- Name: order_items order_items_menu_item_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_items
    ADD CONSTRAINT order_items_menu_item_id_fkey FOREIGN KEY (menu_item_id) REFERENCES public.menu_items(id) ON DELETE CASCADE;


--
-- Name: order_items order_items_order_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.order_items
    ADD CONSTRAINT order_items_order_id_fkey FOREIGN KEY (order_id) REFERENCES public.orders(id) ON DELETE CASCADE;


--
-- Name: orders orders_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: orders orders_vendor_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.orders
    ADD CONSTRAINT orders_vendor_id_fkey FOREIGN KEY (vendor_id) REFERENCES public.vendors(id);


--
-- Name: password_reset_tokens password_reset_tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: sessions sessions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.sessions
    ADD CONSTRAINT sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: vendor_applications vendor_applications_reviewed_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_applications
    ADD CONSTRAINT vendor_applications_reviewed_by_fkey FOREIGN KEY (reviewed_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: vendor_applications vendor_applications_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_applications
    ADD CONSTRAINT vendor_applications_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: vendor_payments vendor_payments_vendor_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendor_payments
    ADD CONSTRAINT vendor_payments_vendor_id_fkey FOREIGN KEY (vendor_id) REFERENCES public.vendors(id) ON DELETE CASCADE;


--
-- Name: vendors vendors_owner_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.vendors
    ADD CONSTRAINT vendors_owner_user_id_fkey FOREIGN KEY (owner_user_id) REFERENCES public.users(id) ON DELETE SET NULL;

-- Added after vendors_pkey exists (a foreign key cannot be created before its target key).
ALTER TABLE ONLY public.vendor_payout_verifications
    ADD CONSTRAINT vendor_payout_verifications_vendor_id_fkey FOREIGN KEY (vendor_id) REFERENCES public.vendors(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

ALTER TABLE ONLY public.phone_verifications
    ADD CONSTRAINT phone_verifications_pkey PRIMARY KEY (pin_id);


ALTER TABLE ONLY public.phone_verifications
    ADD CONSTRAINT phone_verifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


\unrestrict 9GnhRrLzS2qMK4XQomifpIcRweLmEzOeaO7Q0sMZYqZ2NIhOrIvvBMsYZbjKSc7



-- Customer-vendor messaging for ENIE.
CREATE TABLE public.message_threads (
    id integer GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    customer_id integer NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    vendor_id integer NOT NULL REFERENCES public.vendors(id) ON DELETE CASCADE,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT message_threads_customer_vendor_key UNIQUE (customer_id, vendor_id)
);

CREATE TABLE public.messages (
    id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    thread_id integer NOT NULL REFERENCES public.message_threads(id) ON DELETE CASCADE,
    sender_user_id integer NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    delivered_at timestamp with time zone,
    read_at timestamp with time zone
);

CREATE INDEX idx_message_threads_customer_updated ON public.message_threads(customer_id, updated_at DESC);
CREATE INDEX idx_messages_thread_id ON public.messages(thread_id, id);

CREATE INDEX idx_message_threads_vendor_updated ON public.message_threads(vendor_id, updated_at DESC);

-- In-app customer notifications
CREATE TABLE IF NOT EXISTS public.notifications (
    id integer NOT NULL GENERATED BY DEFAULT AS IDENTITY,
    user_id integer NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    type text NOT NULL DEFAULT 'general',
    title text NOT NULL,
    message text NOT NULL,
    link text NOT NULL DEFAULT '',
    read_at timestamp with time zone,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS idx_notifications_user_created ON public.notifications(user_id, created_at DESC);

-- Loyalty points earned from completed customer orders.
CREATE TABLE IF NOT EXISTS public.loyalty_transactions (
    id integer GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    user_id integer NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    order_id integer REFERENCES public.orders(id) ON DELETE SET NULL,
    points integer NOT NULL CHECK (points > 0),
    type text NOT NULL DEFAULT 'earned' CHECK (type IN ('earned', 'redeemed')),
    description text NOT NULL DEFAULT '',
    created_at timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_loyalty_transactions_user_created
    ON public.loyalty_transactions (user_id, created_at DESC);

-- Existing deployments may have a unique order_id constraint from the original
-- loyalty implementation. Redemption and earning both reference the same order,
-- so only earned awards need to be unique per order.
ALTER TABLE public.loyalty_transactions
    DROP CONSTRAINT IF EXISTS loyalty_transactions_order_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS uq_loyalty_earned_order
    ON public.loyalty_transactions (order_id)
    WHERE type = 'earned' AND order_id IS NOT NULL;

ALTER TABLE public.orders
    ADD COLUMN IF NOT EXISTS loyalty_points_redeemed integer NOT NULL DEFAULT 0;
ALTER TABLE public.orders
    DROP CONSTRAINT IF EXISTS orders_loyalty_points_redeemed_check;
ALTER TABLE public.orders
    ADD CONSTRAINT orders_loyalty_points_redeemed_check CHECK (loyalty_points_redeemed >= 0);
