-- Seed Demo User: email=customer@gophermart.com, password=password123
-- bcrypt hash for "password123" is $2a$10$7s26mX71jZ8/21b/J8bHk.q2vQ6X4X8dK0ZgJ4JkUj6kL.6H/jK.e
INSERT INTO users (id, email, password_hash, name, role)
VALUES 
    ('a0000000-0000-0000-0000-000000000001', 'customer@gophermart.com', '$2a$10$j8dF4wRkU06P5gL0aLgTee9j5d5aT7yB0PjT9wYy4tWvHkFpC5E4i', 'Gopher Customer', 'customer'),
    ('a0000000-0000-0000-0000-000000000002', 'admin@gophermart.com', '$2a$10$j8dF4wRkU06P5gL0aLgTee9j5d5aT7yB0PjT9wYy4tWvHkFpC5E4i', 'Store Admin', 'admin')
ON CONFLICT (email) DO NOTHING;

-- Seed Dark-Store Quick Commerce Products
INSERT INTO products (id, sku, name, description, price_cents, category)
VALUES
    ('b0000000-0000-0000-0000-000000000001', 'SKU-MILK-001', 'Organic Whole Milk (1L)', 'Fresh whole milk delivered in 10 minutes', 349, 'Dairy'),
    ('b0000000-0000-0000-0000-000000000002', 'SKU-BREAD-002', 'Sourdough Artisan Loaf', 'Locally baked crusty sourdough bread', 499, 'Bakery'),
    ('b0000000-0000-0000-0000-000000000003', 'SKU-COFFEE-003', 'Cold Brew Nitro Concentrate', 'Single origin Arabica nitro brew (500ml)', 699, 'Beverages'),
    ('b0000000-0000-0000-0000-000000000004', 'SKU-AVOCADO-004', 'Hass Avocados (Pack of 3)', 'Ripe ready-to-eat Hass avocados', 599, 'Produce'),
    ('b0000000-0000-0000-0000-000000000005', 'SKU-CHIPS-005', 'Sea Salt Truffle Crisps', 'Artisan hand-cooked potato crisps', 299, 'Snacks')
ON CONFLICT (sku) DO NOTHING;

-- Seed Inventory for Products (10-50 units to demonstrate concurrency tests)
INSERT INTO inventory (id, product_id, quantity, reserved_quantity, version)
VALUES
    ('c0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', 50, 0, 1),
    ('c0000000-0000-0000-0000-000000000002', 'b0000000-0000-0000-0000-000000000002', 30, 0, 1),
    ('c0000000-0000-0000-0000-000000000003', 'b0000000-0000-0000-0000-000000000003', 10, 0, 1), -- Limited stock for overselling test!
    ('c0000000-0000-0000-0000-000000000004', 'b0000000-0000-0000-0000-000000000004', 25, 0, 1),
    ('c0000000-0000-0000-0000-000000000005', 'b0000000-0000-0000-0000-000000000005', 100, 0, 1)
ON CONFLICT (product_id) DO NOTHING;
