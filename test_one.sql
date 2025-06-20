--users
--id  name

--orders
--id  user_id amount  date

SELECT *
FROM users AS u
JOIN LEFT orders AS o ON o.user_id = u.id
-- WHERE o.amount > 100
GROUP BY o.amount, o.user_id
HAVING amount > 100