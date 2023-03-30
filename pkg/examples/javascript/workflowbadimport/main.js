import airplane from "airplane";
import * as mailer from "nodemailer";

export default async function (params) {
  airplane.setOutput(params.id);

  // Adapted from example provided by user.
  const transport = mailer.createTransport({
    host: "smtp.gmail.com",
    port: 587,
    secure: true,
    auth: {
      user: "yolken@gmail.com",
      pass: "xxx",
    },
  });
  transport
    .sendMail({
      to: "yolken@gmail.com",
      sender: "yolken@gmail.com",
      text: "We are checking unused serials now: Go to https://app.airplane.dev to finish this",
      subject: "Create serial each month",
    })
    .then((info) => {
      console.log(info);
    });
}
